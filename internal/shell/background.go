package shell

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// MaxFinishedJobs bounds how many finished jobs keep their output
	// readable. The oldest are dropped first.
	MaxFinishedJobs = 250
	// CompletedJobRetention is how long a finished job's output stays
	// readable.
	CompletedJobRetention = 8 * time.Hour
)

// JobScope identifies who a background job belongs to. Owner is a
// comparable value for whoever started the job, such as an app instance;
// Session is the session the job serves. A zero Session matches jobs
// started outside any session.
type JobScope struct {
	Owner   any
	Session string
}

// BackgroundShell represents a shell running in the background.
type BackgroundShell struct {
	ID          string
	Command     string
	Description string
	Shell       *Shell
	WorkingDir  string
	StartedAt   time.Time
	scope       JobScope
	ctx         context.Context
	cancel      context.CancelFunc
	stdout      *cappedBuffer
	stderr      *cappedBuffer
	done        chan struct{}
	exitErr     error
	completedAt atomic.Int64 // Unix timestamp when job completed (0 if still running)
	background  atomic.Bool
	killed      atomic.Bool // Kill was called; forget the shell once it exits
}

// BackgroundShellManager manages background shell instances.
type BackgroundShellManager struct {
	mu     sync.Mutex
	shells map[string]*BackgroundShell
}

var (
	backgroundManager     *BackgroundShellManager
	backgroundManagerOnce sync.Once
	idCounter             atomic.Uint64
)

// newBackgroundShellManager creates a new BackgroundShellManager instance.
func newBackgroundShellManager() *BackgroundShellManager {
	return &BackgroundShellManager{shells: map[string]*BackgroundShell{}}
}

// GetBackgroundShellManager returns the singleton background shell manager.
func GetBackgroundShellManager() *BackgroundShellManager {
	backgroundManagerOnce.Do(func() {
		backgroundManager = newBackgroundShellManager()
	})
	return backgroundManager
}

// Start creates and starts a new background shell with the given command,
// unless maxJobs jobs of the same scope are already running, so one busy
// session cannot use up every other session's slots. The shell is tagged
// with scope,
// so lookups can be limited to it and [BackgroundShellManager.KillOwned]
// can stop it with its owner's other shells.
func (m *BackgroundShellManager) Start(ctx context.Context, scope JobScope, workingDir string, blockFuncs []BlockFunc, command, description string, maxJobs int) (*BackgroundShell, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prune()
	if m.runningIn(scope) >= maxJobs {
		return nil, fmt.Errorf("maximum number of running background jobs in this session (%d) reached. Please terminate or wait for some jobs to complete", maxJobs)
	}

	id := fmt.Sprintf("%03X", idCounter.Add(1))

	shell := NewShell(&Options{
		WorkingDir: workingDir,
		BlockFuncs: blockFuncs,
	})

	shellCtx, cancel := context.WithCancel(ctx)

	bgShell := &BackgroundShell{
		ID:          id,
		Command:     command,
		Description: description,
		WorkingDir:  workingDir,
		Shell:       shell,
		StartedAt:   time.Now(),
		scope:       scope,
		ctx:         shellCtx,
		cancel:      cancel,
		stdout:      newCappedBuffer(maxKeptOutput),
		stderr:      newCappedBuffer(maxKeptOutput),
		done:        make(chan struct{}),
	}

	m.shells[id] = bgShell

	go func() {
		defer func() {
			close(bgShell.done)
			if bgShell.killed.Load() {
				m.forget(bgShell)
			}
		}()

		err := shell.ExecStream(shellCtx, command, bgShell.stdout, bgShell.stderr)

		bgShell.exitErr = err
		bgShell.completedAt.Store(time.Now().Unix())
	}()

	return bgShell, nil
}

// runningIn counts the shells of scope that are still running, foreground
// ones included, since each holds a slot. Callers must hold m.mu.
func (m *BackgroundShellManager) runningIn(scope JobScope) int {
	running := 0
	for _, shell := range m.shells {
		if shell.scope == scope && shell.completedAt.Load() == 0 {
			running++
		}
	}
	return running
}

// prune drops finished jobs that are past retention or beyond
// MaxFinishedJobs. Callers must hold m.mu.
func (m *BackgroundShellManager) prune() {
	cutoff := time.Now().Add(-CompletedJobRetention).Unix()
	var finished []*BackgroundShell
	for id, shell := range m.shells {
		switch at := shell.completedAt.Load(); {
		case at == 0:
		case at < cutoff:
			delete(m.shells, id)
		default:
			finished = append(finished, shell)
		}
	}

	if over := len(finished) - MaxFinishedJobs; over > 0 {
		slices.SortFunc(finished, func(a, b *BackgroundShell) int {
			return cmp.Compare(a.completedAt.Load(), b.completedAt.Load())
		})
		for _, shell := range finished[:over] {
			delete(m.shells, shell.ID)
		}
	}
}

// take removes and returns the shell with the given ID.
func (m *BackgroundShellManager) take(id string) (*BackgroundShell, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	shell, ok := m.shells[id]
	if !ok {
		return nil, fmt.Errorf("background shell not found: %s", id)
	}
	delete(m.shells, id)
	return shell, nil
}

// forget stops tracking shell, unless its ID now names a different shell.
func (m *BackgroundShellManager) forget(shell *BackgroundShell) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shells[shell.ID] == shell {
		delete(m.shells, shell.ID)
	}
}

// Get retrieves a background shell by ID.
func (m *BackgroundShellManager) Get(id string) (*BackgroundShell, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	shell, ok := m.shells[id]
	return shell, ok
}

// Lookup retrieves a background shell by ID, but only if it belongs to
// scope.
func (m *BackgroundShellManager) Lookup(scope JobScope, id string) (*BackgroundShell, bool) {
	shell, ok := m.Get(id)
	if !ok || shell.scope != scope {
		return nil, false
	}
	return shell, true
}

// Jobs returns the background shells that belong to scope, running and
// finished, oldest first. Shells still running in the foreground and
// shells being killed are left out.
func (m *BackgroundShellManager) Jobs(scope JobScope) []*BackgroundShell {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prune()
	var jobs []*BackgroundShell
	for _, shell := range m.shells {
		if shell.listed(scope) {
			jobs = append(jobs, shell)
		}
	}
	slices.SortFunc(jobs, func(a, b *BackgroundShell) int {
		return a.StartedAt.Compare(b.StartedAt)
	})
	return jobs
}

// Running reports how many background shells that belong to scope are
// still running.
func (m *BackgroundShellManager) Running(scope JobScope) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	running := 0
	for _, shell := range m.shells {
		if shell.listed(scope) && !shell.IsDone() {
			running++
		}
	}
	return running
}

// Remove stops tracking a background shell without terminating it.
func (m *BackgroundShellManager) Remove(id string) error {
	_, err := m.take(id)
	return err
}

// killWaitTimeout bounds how long Kill waits for a cancelled shell to
// report done before giving up and returning.
const killWaitTimeout = 5 * time.Second

// Kill terminates a background shell by ID. It never blocks indefinitely:
// a stubborn descendant can keep the shell's pipes open past cancellation,
// so it stops waiting once ctx is done or killWaitTimeout elapses. Either
// way the shell stays tracked until it exits, so shutdown can still wait
// for its cleanup, and is forgotten as soon as it does.
func (m *BackgroundShellManager) Kill(ctx context.Context, id string) error {
	shell, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}
	shell.killed.Store(true)
	shell.cancel()
	timer := time.NewTimer(killWaitTimeout)
	defer timer.Stop()
	select {
	case <-shell.done:
	case <-ctx.Done():
	case <-timer.C:
	}
	if !shell.IsDone() {
		return errors.New("kill signal sent, process may still be exiting")
	}
	m.forget(shell)
	return nil
}

// BackgroundShellInfo contains information about a background shell.
type BackgroundShellInfo struct {
	ID          string
	Command     string
	Description string
}

// List returns all background shell IDs.
func (m *BackgroundShellManager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Collect(maps.Keys(m.shells))
}

// KillAll terminates all background shells. The provided context bounds how
// long the function waits for each shell to exit.
func (m *BackgroundShellManager) KillAll(ctx context.Context) {
	m.mu.Lock()
	shells := m.shells
	m.shells = map[string]*BackgroundShell{}
	m.mu.Unlock()

	killShells(ctx, shells)
}

// KillOwned terminates the background shells started on behalf of owner,
// leaving every other owner's shells running. The provided context bounds
// how long the function waits for each shell to exit.
func (m *BackgroundShellManager) KillOwned(ctx context.Context, owner any) {
	m.mu.Lock()
	shells := map[string]*BackgroundShell{}
	for id, shell := range m.shells {
		if shell.scope.Owner == owner {
			shells[id] = shell
			delete(m.shells, id)
		}
	}
	m.mu.Unlock()

	killShells(ctx, shells)
}

// killShells cancels the given shells and waits for each to exit, or for
// ctx to be done.
func killShells(ctx context.Context, shells map[string]*BackgroundShell) {
	var wg sync.WaitGroup
	for _, shell := range shells {
		wg.Go(func() {
			shell.cancel()
			select {
			case <-shell.done:
			case <-ctx.Done():
			}
		})
	}
	wg.Wait()
}

// GetOutput returns the current output of a background shell.
func (bs *BackgroundShell) GetOutput() (stdout string, stderr string, done bool, err error) {
	select {
	case <-bs.done:
		return bs.stdout.String(), bs.stderr.String(), true, bs.exitErr
	default:
		return bs.stdout.String(), bs.stderr.String(), false, nil
	}
}

// MarkBackground records that the shell now runs in the background, so it
// shows up in [BackgroundShellManager.Jobs] and
// [BackgroundShellManager.Running].
func (bs *BackgroundShell) MarkBackground() {
	bs.background.Store(true)
}

// listed reports whether the shell is a background job of scope that has
// not been killed.
func (bs *BackgroundShell) listed(scope JobScope) bool {
	return bs.scope == scope && bs.background.Load() && !bs.killed.Load()
}

// CompletedAt returns when the background shell finished, or the zero time
// while it is still running.
func (bs *BackgroundShell) CompletedAt() time.Time {
	if at := bs.completedAt.Load(); at != 0 {
		return time.Unix(at, 0)
	}
	return time.Time{}
}

// IsDone checks if the background shell has finished execution.
func (bs *BackgroundShell) IsDone() bool {
	select {
	case <-bs.done:
		return true
	default:
		return false
	}
}

// Done returns a channel that is closed when the background shell exits.
func (bs *BackgroundShell) Done() <-chan struct{} {
	return bs.done
}

// Wait blocks until the background shell completes.
func (bs *BackgroundShell) Wait() {
	<-bs.done
}

func (bs *BackgroundShell) WaitContext(ctx context.Context) bool {
	select {
	case <-bs.done:
		return true
	case <-ctx.Done():
		return false
	}
}
