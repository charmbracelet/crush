package shell

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
)

const (
	// MaxBackgroundJobs is the maximum number of concurrent background jobs allowed
	MaxBackgroundJobs = 50
	// MaxOutputBufferSize is the maximum size of output buffer per stream (1MB)
	MaxOutputBufferSize = 1024 * 1024
	// CompletedJobRetentionMinutes is how long to keep completed jobs before auto-cleanup
	CompletedJobRetentionMinutes = 30
)

// LimitedBuffer is a buffer that enforces a maximum size by truncating old data
type LimitedBuffer struct {
	buf       *bytes.Buffer
	maxSize   int
	totalSize int
	mu        sync.Mutex
	truncated bool
}

// NewLimitedBuffer creates a new limited buffer with the specified max size
func NewLimitedBuffer(maxSize int) *LimitedBuffer {
	return &LimitedBuffer{
		buf:     &bytes.Buffer{},
		maxSize: maxSize,
	}
}

// Write implements io.Writer and enforces the size limit
func (lb *LimitedBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.totalSize += len(p)

	// If adding this data would exceed the limit, truncate from the beginning
	if lb.buf.Len()+len(p) > lb.maxSize {
		lb.truncated = true
		// Calculate how much to keep from the current buffer
		keepSize := lb.maxSize - len(p)
		if keepSize <= 0 {
			// New data alone exceeds limit, just take the end of new data
			lb.buf.Reset()
			if len(p) > lb.maxSize {
				lb.buf.Write(p[len(p)-lb.maxSize:])
				return len(p), nil
			}
		} else {
			// Keep the most recent data from buffer
			currentData := lb.buf.Bytes()
			lb.buf.Reset()
			lb.buf.Write(currentData[len(currentData)-keepSize:])
		}
	}

	return lb.buf.Write(p)
}

// String returns the buffered content with truncation notice if applicable
func (lb *LimitedBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.truncated {
		truncatedBytes := lb.totalSize - lb.buf.Len()
		notice := fmt.Sprintf("... [output truncated: %d bytes omitted] ...\n\n", truncatedBytes)
		return notice + lb.buf.String()
	}
	return lb.buf.String()
}

// Len returns the current buffer length
func (lb *LimitedBuffer) Len() int {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Len()
}

// BackgroundShell represents a shell running in the background.
type BackgroundShell struct {
	ID           string
	Command      string
	Description  string
	Shell        *Shell
	WorkingDir   string
	ctx          context.Context
	cancel       context.CancelFunc
	stdout       *LimitedBuffer
	stderr       *LimitedBuffer
	done         chan struct{}
	exitErr      error
	completedAt  int64 // Unix timestamp when job completed (0 if still running)
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

// GetBackgroundShellManager returns the singleton background shell manager.
func GetBackgroundShellManager() *BackgroundShellManager {
	backgroundManagerOnce.Do(func() {
		backgroundManager = &BackgroundShellManager{
			shells: csync.NewMap[string, *BackgroundShell](),
		}
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
		stdout:      NewLimitedBuffer(MaxOutputBufferSize),
		stderr:      NewLimitedBuffer(MaxOutputBufferSize),
		done:        make(chan struct{}),
	}

	m.shells.Set(id, bgShell)

	go func() {
		defer close(bgShell.done)

		err := shell.ExecStream(shellCtx, command, bgShell.stdout, bgShell.stderr)

		bgShell.exitErr = err
		atomic.StoreInt64(&bgShell.completedAt, time.Now().Unix())
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
		completedAt := atomic.LoadInt64(&shell.completedAt)
		if completedAt > 0 && now-completedAt > retentionSeconds {
			toRemove = append(toRemove, shell.ID)
		}
	}

	for _, id := range toRemove {
		m.Remove(id)
	}

	return len(toRemove)
}

// KillAll terminates all background shells.
func (m *BackgroundShellManager) KillAll() {
	shells := make([]*BackgroundShell, 0, m.shells.Len())
	for shell := range m.shells.Seq() {
		shells = append(shells, shell)
	}
	m.shells.Reset(map[string]*BackgroundShell{})

	for _, shell := range shells {
		shell.cancel()
		<-shell.done
	}
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
