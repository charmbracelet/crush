package shell

import "sync"

// foregroundWait tracks a bash tool that is still blocking the agent turn
// while its shell already runs under BackgroundShellManager. Releasing the
// wait lets the tool return a background job result without killing the
// process (Ctrl+B / manual background).
type foregroundWait struct {
	shellID string
	release chan struct{}
	once    sync.Once
}

type foregroundWaitRegistry struct {
	mu sync.Mutex
	// sessionID -> shellID -> wait
	waits map[string]map[string]*foregroundWait
}

var (
	fgWaitRegistry     *foregroundWaitRegistry
	fgWaitRegistryOnce sync.Once
)

func getForegroundWaitRegistry() *foregroundWaitRegistry {
	fgWaitRegistryOnce.Do(func() {
		fgWaitRegistry = &foregroundWaitRegistry{
			waits: make(map[string]map[string]*foregroundWait),
		}
	})
	return fgWaitRegistry
}

// RegisterForegroundWait marks a shell as foreground-blocking for the
// session and returns a channel that closes when the wait is released.
// Safe to call more than once for the same shell; subsequent calls return
// the existing channel.
func RegisterForegroundWait(sessionID, shellID string) <-chan struct{} {
	return getForegroundWaitRegistry().register(sessionID, shellID)
}

// UnregisterForegroundWait drops a foreground wait registration. Call this
// when the tool finishes its wait loop (completed, timed out, canceled, or
// already released).
func UnregisterForegroundWait(sessionID, shellID string) {
	getForegroundWaitRegistry().unregister(sessionID, shellID)
}

// ReleaseForegroundWaits closes every foreground wait for the session so
// blocked bash tools return as background jobs. Returns how many waits were
// released. The shells keep running.
func ReleaseForegroundWaits(sessionID string) int {
	return getForegroundWaitRegistry().releaseSession(sessionID)
}

// HasForegroundWaits reports whether the session has any bash tools still
// blocking the agent turn that can be manually backgrounded.
func HasForegroundWaits(sessionID string) bool {
	return getForegroundWaitRegistry().hasSession(sessionID)
}

func (r *foregroundWaitRegistry) register(sessionID, shellID string) <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	if sessionID == "" || shellID == "" {
		// Always-open channel: never releases. Callers with missing IDs
		// simply cannot be manually backgrounded.
		ch := make(chan struct{})
		return ch
	}

	if r.waits[sessionID] == nil {
		r.waits[sessionID] = make(map[string]*foregroundWait)
	}
	if w, ok := r.waits[sessionID][shellID]; ok {
		return w.release
	}
	w := &foregroundWait{
		shellID: shellID,
		release: make(chan struct{}),
	}
	r.waits[sessionID][shellID] = w
	return w.release
}

func (r *foregroundWaitRegistry) unregister(sessionID, shellID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.waits[sessionID]
	if !ok {
		return
	}
	delete(m, shellID)
	if len(m) == 0 {
		delete(r.waits, sessionID)
	}
}

func (r *foregroundWaitRegistry) releaseSession(sessionID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	m := r.waits[sessionID]
	if len(m) == 0 {
		return 0
	}
	n := 0
	for _, w := range m {
		w.once.Do(func() { close(w.release) })
		n++
	}
	delete(r.waits, sessionID)
	return n
}

func (r *foregroundWaitRegistry) hasSession(sessionID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.waits[sessionID]) > 0
}
