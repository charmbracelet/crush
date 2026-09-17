package shell

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
)

const (
	// DefaultMaxBackgroundJobs is how many background jobs may run at once
	// when nothing else is configured. Finished jobs never count against it.
	DefaultMaxBackgroundJobs = 50
	// MaxRetainedJobs bounds how many jobs are tracked in total, including
	// finished ones whose output can still be read. The oldest go first.
	MaxRetainedJobs = 250
	// CompletedJobRetentionMinutes is how long a finished job's output stays
	// readable (8 hours).
	CompletedJobRetentionMinutes = 8 * 60
)

// syncBuffer is a thread-safe wrapper around bytes.Buffer.
type syncBuffer struct {
	buf bytes.Buffer
	mu  sync.RWMutex
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuffer) WriteString(s string) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.WriteString(s)
}

func (sb *syncBuffer) String() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.String()
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
	// admit serializes the capacity check in Start so concurrent callers
	// cannot both pass the limit and then both register a shell.
	admit sync.Mutex
	// maxJobs is the configured ceiling on running jobs. Zero means
	// DefaultMaxBackgroundJobs.
	maxJobs atomic.Int64
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

// SetMaxJobs sets how many jobs may run at once. Jobs already running are
// left alone, so a lowered limit only bites on the next start.
func (m *BackgroundShellManager) SetMaxJobs(n int) {
	m.maxJobs.Store(int64(n))
}

// MaxJobs reports the ceiling on running jobs.
func (m *BackgroundShellManager) MaxJobs() int {
	if n := m.maxJobs.Load(); n > 0 {
		return int(n)
	}
	return DefaultMaxBackgroundJobs
}

// Start creates and starts a new background shell with the given command.
func (m *BackgroundShellManager) Start(ctx context.Context, workingDir string, blockFuncs []BlockFunc, command string, description string) (*BackgroundShell, error) {
	m.admit.Lock()
	defer m.admit.Unlock()

	// Only jobs that are still running hold a slot. Finished jobs stay in the
	// map so their output remains readable, but they must not block new work.
	limit := m.MaxJobs()
	if running := m.runningCount(); running >= limit {
		return nil, fmt.Errorf("maximum number of running background jobs (%d) reached. Please terminate or wait for some jobs to complete", limit)
	}

	m.dropStale()
	m.trim(limit, 1)

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
		stdout:      &syncBuffer{},
		stderr:      &syncBuffer{},
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

// runningCount reports how many tracked jobs have not finished yet.
func (m *BackgroundShellManager) runningCount() int {
	var n int
	for shell := range m.shells.Seq() {
		if shell.completedAt.Load() == 0 {
			n++
		}
	}
	return n
}

// trim drops the oldest finished jobs so that headroom more jobs can be
// tracked without passing the retention cap. Running jobs are never dropped,
// so the cap has to leave room for a full complement of them.
func (m *BackgroundShellManager) trim(limit, headroom int) {
	retained := max(MaxRetainedJobs, 2*limit)
	over := m.shells.Len() + headroom - retained
	if over <= 0 {
		return
	}

	var done []*BackgroundShell
	for shell := range m.shells.Seq() {
		if shell.completedAt.Load() > 0 {
			done = append(done, shell)
		}
	}
	slices.SortFunc(done, func(a, b *BackgroundShell) int {
		return cmp.Compare(a.completedAt.Load(), b.completedAt.Load())
	})

	for _, job := range done[:min(over, len(done))] {
		m.shells.Del(job.ID)
	}
}

// dropStale removes finished jobs whose output has been readable for longer
// than the retention period.
func (m *BackgroundShellManager) dropStale() {
	cutoff := time.Now().Unix() - int64(CompletedJobRetentionMinutes*60)
	for shell := range m.shells.Seq() {
		if at := shell.completedAt.Load(); at > 0 && at < cutoff {
			m.shells.Del(shell.ID)
		}
	}
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

// KillAll terminates all background shells. The provided context bounds how
// long the function waits for each shell to exit.
func (m *BackgroundShellManager) KillAll(ctx context.Context) {
	// Held across the clear so a Start that has already passed the capacity
	// check cannot register its shell behind us and survive the kill.
	m.admit.Lock()
	shells := slices.Collect(m.shells.Seq())
	m.shells.Reset(map[string]*BackgroundShell{})
	m.admit.Unlock()

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
