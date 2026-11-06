package shell

import (
	"cmp"
	"context"
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

// BackgroundShell represents a shell running in the background.
type BackgroundShell struct {
	ID          string
	Command     string
	Description string
	Shell       *Shell
	WorkingDir  string
	ctx         context.Context
	cancel      context.CancelFunc
	stdout      *cappedBuffer
	stderr      *cappedBuffer
	done        chan struct{}
	exitErr     error
	completedAt atomic.Int64 // Unix timestamp when job completed (0 if still running)
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
// unless maxJobs jobs are already running.
func (m *BackgroundShellManager) Start(ctx context.Context, workingDir string, blockFuncs []BlockFunc, command, description string, maxJobs int) (*BackgroundShell, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if running := m.prune(); running >= maxJobs {
		return nil, fmt.Errorf("maximum number of running background jobs (%d) reached. Please terminate or wait for some jobs to complete", maxJobs)
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
		stdout:      newCappedBuffer(maxKeptOutput),
		stderr:      newCappedBuffer(maxKeptOutput),
		done:        make(chan struct{}),
	}

	m.shells[id] = bgShell

	go func() {
		defer close(bgShell.done)

		err := shell.ExecStream(shellCtx, command, bgShell.stdout, bgShell.stderr)

		bgShell.exitErr = err
		bgShell.completedAt.Store(time.Now().Unix())
	}()

	return bgShell, nil
}

// prune drops finished jobs that are past retention or beyond
// MaxFinishedJobs, and reports how many jobs are still running. Callers
// must hold m.mu.
func (m *BackgroundShellManager) prune() (running int) {
	cutoff := time.Now().Add(-CompletedJobRetention).Unix()
	var finished []*BackgroundShell
	for id, shell := range m.shells {
		switch at := shell.completedAt.Load(); {
		case at == 0:
			running++
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
	return running
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

// Get retrieves a background shell by ID.
func (m *BackgroundShellManager) Get(id string) (*BackgroundShell, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	shell, ok := m.shells[id]
	return shell, ok
}

// Remove stops tracking a background shell without terminating it.
func (m *BackgroundShellManager) Remove(id string) error {
	_, err := m.take(id)
	return err
}

// Kill terminates a background shell by ID.
func (m *BackgroundShellManager) Kill(id string) error {
	shell, err := m.take(id)
	if err != nil {
		return err
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
