package shell

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/csync"
)

const (
	// MaxBackgroundJobs is the maximum number of concurrent background jobs allowed
	MaxBackgroundJobs = 50
	// CompletedJobRetentionMinutes is how long to keep completed jobs before auto-cleanup (8 hours)
	CompletedJobRetentionMinutes = 8 * 60

	defaultSyncBufferHeadBytes = 1 << 20 // 1 MiB
	defaultSyncBufferTailBytes = 1 << 20 // 1 MiB
)

// syncBuffer is a thread-safe output buffer that retains a fixed head and a
// rolling tail once the stream exceeds head+tail bytes. A single large Write
// never retains more than head+tail bytes of payload.
//
// The tail is a fixed-capacity ring buffer, so a write costs O(len(p)) and
// allocates nothing once the ring exists. Re-slicing the tail on every write
// instead would copy and re-allocate tailLimit bytes per write, making the
// cost of draining a noisy child O(total output) rather than O(1) amortized,
// all of it under the write lock that String readers contend on.
type syncBuffer struct {
	mu        sync.RWMutex
	headLimit int
	tailLimit int
	head      []byte
	tail      []byte // ring of exactly tailLimit bytes, allocated on first use
	tailStart int    // index of the oldest retained byte in tail
	tailLen   int    // number of valid bytes in the ring
	total     int64
	truncated bool
}

func newSyncBuffer() *syncBuffer {
	return &syncBuffer{
		headLimit: defaultSyncBufferHeadBytes,
		tailLimit: defaultSyncBufferTailBytes,
	}
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.writeLocked(p)
	return len(p), nil
}

func (sb *syncBuffer) WriteString(s string) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.writeLocked([]byte(s))
	return len(s), nil
}

// ensureLimits fills in the default window for a zero-value syncBuffer. Without
// it a syncBuffer{} constructed outside newSyncBuffer has headLimit and
// tailLimit of 0 and silently discards every byte written to it.
func (sb *syncBuffer) ensureLimits() {
	if sb.headLimit <= 0 && sb.tailLimit <= 0 {
		sb.headLimit = defaultSyncBufferHeadBytes
		sb.tailLimit = defaultSyncBufferTailBytes
	}
}

func (sb *syncBuffer) writeLocked(p []byte) {
	if len(p) == 0 {
		return
	}
	sb.ensureLimits()
	sb.total += int64(len(p))

	if len(sb.head) < sb.headLimit {
		need := sb.headLimit - len(sb.head)
		if len(p) <= need {
			sb.head = append(sb.head, p...)
			return
		}
		sb.head = append(sb.head, p[:need]...)
		p = p[need:]
	}
	if len(p) == 0 || sb.tailLimit <= 0 {
		if len(p) > 0 {
			sb.truncated = true
		}
		return
	}

	if sb.tail == nil {
		sb.tail = make([]byte, sb.tailLimit)
	}

	// Keep only the last tailLimit bytes of everything past the head, writing
	// into a fixed ring so nothing is allocated or re-copied per write.
	if len(p) >= sb.tailLimit {
		// Prior tail content (if any) is fully superseded by this write.
		if sb.tailLen > 0 {
			sb.truncated = true
		}
		copy(sb.tail, p[len(p)-sb.tailLimit:])
		sb.tailStart = 0
		sb.tailLen = sb.tailLimit
	} else {
		w := (sb.tailStart + sb.tailLen) % sb.tailLimit
		n := copy(sb.tail[w:], p)
		if n < len(p) {
			copy(sb.tail, p[n:])
		}
		if sb.tailLen+len(p) > sb.tailLimit {
			// The write wrapped past the oldest byte; advance the start.
			over := sb.tailLen + len(p) - sb.tailLimit
			sb.tailStart = (sb.tailStart + over) % sb.tailLimit
			sb.tailLen = sb.tailLimit
			sb.truncated = true
		} else {
			sb.tailLen += len(p)
		}
	}
	// Marker only when the retained window is smaller than total written.
	if sb.total > int64(len(sb.head)+sb.tailLen) {
		sb.truncated = true
	}
}

// tailSegments returns the retained tail as up to two slices, oldest first.
func (sb *syncBuffer) tailSegments() ([]byte, []byte) {
	if sb.tailLen == 0 {
		return nil, nil
	}
	end := sb.tailStart + sb.tailLen
	if end <= sb.tailLimit {
		return sb.tail[sb.tailStart:end], nil
	}
	return sb.tail[sb.tailStart:sb.tailLimit], sb.tail[:end-sb.tailLimit]
}

// trimPartialRuneSuffix drops a trailing byte sequence that is the start of a
// multi-byte rune the truncation point cut in half. Without it the head ends
// mid-rune and renders as U+FFFD.
func trimPartialRuneSuffix(b []byte) []byte {
	for i := len(b) - 1; i >= 0 && i >= len(b)-utf8.UTFMax; i-- {
		if !utf8.RuneStart(b[i]) {
			continue
		}
		if r, size := utf8.DecodeRune(b[i:]); r == utf8.RuneError && size <= 1 {
			return b[:i]
		}
		return b
	}
	return b
}

// trimPartialRunePrefix drops leading continuation bytes left behind when the
// truncation point cut a multi-byte rune in half.
func trimPartialRunePrefix(b []byte) []byte {
	for i := 0; i < len(b) && i < utf8.UTFMax; i++ {
		if utf8.RuneStart(b[i]) {
			return b[i:]
		}
	}
	return b
}

func (sb *syncBuffer) String() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	head := sb.head
	t1, t2 := sb.tailSegments()

	// Only trim rune fragments at boundaries the truncation actually cut. An
	// untruncated stream is returned byte-for-byte, including trailing bytes of
	// a rune whose remainder simply has not been written yet.
	if sb.truncated {
		head = trimPartialRuneSuffix(head)
		if len(t1) > 0 {
			t1 = trimPartialRunePrefix(t1)
		} else {
			t2 = trimPartialRunePrefix(t2)
		}
	}

	tailLen := len(t1) + len(t2)
	omitted := sb.total - int64(len(head)) - int64(tailLen)
	if omitted < 0 {
		omitted = 0
	}

	var b strings.Builder
	if omitted == 0 {
		b.Grow(len(head) + tailLen)
		b.Write(head)
		b.Write(t1)
		b.Write(t2)
		return b.String()
	}
	b.Grow(len(head) + tailLen + 64)
	b.Write(head)
	fmt.Fprintf(&b, "\n\n... [%d bytes truncated] ...\n\n", omitted)
	b.Write(t1)
	b.Write(t2)
	return b.String()
}

// BackgroundShell represents a shell running in the background.
type BackgroundShell struct {
	ID          string
	Command     string
	Description string
	Shell       *Shell
	WorkingDir  string
	ctx         context.Context
	cancel      context.CancelFunc
	stdout      *syncBuffer
	stderr      *syncBuffer
	done        chan struct{}
	exitErr     error
	completedAt atomic.Int64 // Unix timestamp when job completed (0 if still running)
}

// BackgroundShellManager manages background shell instances.
type BackgroundShellManager struct {
	shells *csync.Map[string, *BackgroundShell]
}

var (
	backgroundManager     *BackgroundShellManager
	backgroundManagerOnce sync.Once
	idCounter             atomic.Uint64
)

// newBackgroundShellManager creates a new BackgroundShellManager instance.
func newBackgroundShellManager() *BackgroundShellManager {
	return &BackgroundShellManager{
		shells: csync.NewMap[string, *BackgroundShell](),
	}
}

// GetBackgroundShellManager returns the singleton background shell manager.
func GetBackgroundShellManager() *BackgroundShellManager {
	backgroundManagerOnce.Do(func() {
		backgroundManager = newBackgroundShellManager()
	})
	return backgroundManager
}

// Start creates and starts a new background shell with the given command.
func (m *BackgroundShellManager) Start(ctx context.Context, workingDir string, blockFuncs []BlockFunc, command string, description string) (*BackgroundShell, error) {
	// Check job limit
	if m.shells.Len() >= MaxBackgroundJobs {
		return nil, fmt.Errorf("maximum number of background jobs (%d) reached. Please terminate or wait for some jobs to complete", MaxBackgroundJobs)
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
		ctx:         shellCtx,
		cancel:      cancel,
		stdout:      newSyncBuffer(),
		stderr:      newSyncBuffer(),
		done:        make(chan struct{}),
	}

	m.shells.Set(id, bgShell)

	go func() {
		defer close(bgShell.done)

		err := shell.ExecStream(shellCtx, command, bgShell.stdout, bgShell.stderr)

		bgShell.exitErr = err
		bgShell.completedAt.Store(time.Now().Unix())
	}()

	return bgShell, nil
}

// Get retrieves a background shell by ID.
func (m *BackgroundShellManager) Get(id string) (*BackgroundShell, bool) {
	return m.shells.Get(id)
}

// Remove removes a background shell from the manager without terminating it.
// This is useful when a shell has already completed and you just want to clean up tracking.
func (m *BackgroundShellManager) Remove(id string) error {
	_, ok := m.shells.Take(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}
	return nil
}

// Kill terminates a background shell by ID.
func (m *BackgroundShellManager) Kill(id string) error {
	shell, ok := m.shells.Take(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}

	shell.cancel()
	<-shell.done
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
	ids := make([]string, 0, m.shells.Len())
	for id := range m.shells.Seq2() {
		ids = append(ids, id)
	}
	return ids
}

// Cleanup removes completed jobs that have been finished for more than the retention period
func (m *BackgroundShellManager) Cleanup() int {
	now := time.Now().Unix()
	retentionSeconds := int64(CompletedJobRetentionMinutes * 60)

	var toRemove []string
	for shell := range m.shells.Seq() {
		completedAt := shell.completedAt.Load()
		if completedAt > 0 && now-completedAt > retentionSeconds {
			toRemove = append(toRemove, shell.ID)
		}
	}

	for _, id := range toRemove {
		m.Remove(id)
	}

	return len(toRemove)
}

// KillAll terminates all background shells. The provided context bounds how
// long the function waits for each shell to exit.
func (m *BackgroundShellManager) KillAll(ctx context.Context) {
	shells := slices.Collect(m.shells.Seq())
	m.shells.Reset(map[string]*BackgroundShell{})

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

// IsDone checks if the background shell has finished execution.
func (bs *BackgroundShell) IsDone() bool {
	select {
	case <-bs.done:
		return true
	default:
		return false
	}
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
