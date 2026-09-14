package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
)

const (
	// MaxBackgroundJobs is the maximum number of concurrent background jobs allowed
	MaxBackgroundJobs = 50
	// CompletedJobRetentionMinutes is how long to keep completed jobs before auto-cleanup (8 hours)
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

// syncFile serializes writes to the combined log so stdout and stderr
// interleave the way a terminal would show them. Writes continue to be
// accepted after Close so a late write from a torn-down process cannot
// panic on a closed file.
type syncFile struct {
	mu     sync.Mutex
	f      *os.File
	closed bool
}

func (sf *syncFile) Write(p []byte) (int, error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.closed {
		return len(p), nil
	}
	return sf.f.Write(p)
}

func (sf *syncFile) Close() error {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.closed {
		return nil
	}
	sf.closed = true
	return sf.f.Close()
}

// BackgroundShell represents a shell running in the background.
type BackgroundShell struct {
	ID          string
	Command     string
	Description string
	Shell       *Shell
	WorkingDir  string
	// SessionID records which session started the job, so a completion
	// report can be delivered back to the conversation that asked for it.
	SessionID string
	// StartedAt is when the job was launched.
	StartedAt time.Time
	// LogPath is the combined stdout/stderr transcript on disk. Empty when
	// the log file could not be created, in which case callers fall back to
	// the in-memory buffers.
	LogPath     string
	ctx         context.Context
	cancel      context.CancelFunc
	stdout      *syncBuffer
	stderr      *syncBuffer
	log         *syncFile
	done        chan struct{}
	exitErr     error
	completedAt atomic.Int64 // Unix timestamp when job completed (0 if still running)
	killed      atomic.Bool  // set when the job was terminated on request
	// set once the starting caller stopped waiting and let the job run on
	// its own, which is what makes its completion worth reporting.
	backgrounded atomic.Bool
}

// Killed reports whether the job was terminated by Kill or KillAll rather
// than exiting on its own. Completion handlers use this to stay quiet about
// jobs the caller deliberately stopped.
func (bs *BackgroundShell) Killed() bool {
	return bs.killed.Load()
}

// MarkBackgrounded records that the caller gave up waiting and handed the job
// off to run on its own. Every job starts life in the foreground, so this is
// what separates a command that genuinely went to the background from one
// that merely finished quickly and was reported inline.
func (bs *BackgroundShell) MarkBackgrounded() {
	bs.backgrounded.Store(true)
}

// Backgrounded reports whether the job outlived the caller that started it.
func (bs *BackgroundShell) Backgrounded() bool {
	return bs.backgrounded.Load()
}

// CompletedAt returns when the job finished, or the zero time if it is
// still running.
func (bs *BackgroundShell) CompletedAt() time.Time {
	sec := bs.completedAt.Load()
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// CompletionFunc is called once a background job finishes, after its done
// channel is closed so the shell already reads as complete.
type CompletionFunc func(*BackgroundShell)

// BackgroundShellManager manages background shell instances.
type BackgroundShellManager struct {
	shells     *csync.Map[string, *BackgroundShell]
	logDir     *csync.Value[string]
	onComplete *csync.Value[CompletionFunc]
}

var (
	backgroundManager     *BackgroundShellManager
	backgroundManagerOnce sync.Once
	idCounter             atomic.Uint64
)

// newBackgroundShellManager creates a new BackgroundShellManager instance.
func newBackgroundShellManager() *BackgroundShellManager {
	return &BackgroundShellManager{
		shells:     csync.NewMap[string, *BackgroundShell](),
		logDir:     csync.NewValue(defaultJobLogDir()),
		onComplete: csync.NewValue[CompletionFunc](nil),
	}
}

// defaultJobLogDir is used until SetLogDir points the manager at the
// configured data directory. Keeping a usable default means job logs and
// incremental reads work even when nothing wired the manager up.
func defaultJobLogDir() string {
	return filepath.Join(os.TempDir(), "crush-jobs")
}

// SetLogDir chooses where combined job transcripts are written. Passing an
// empty string restores the default temporary directory.
func (m *BackgroundShellManager) SetLogDir(dir string) {
	if dir == "" {
		dir = defaultJobLogDir()
	}
	m.logDir.Set(dir)
}

// SetCompletionHandler registers the function called when a background job
// finishes. Passing nil disables reporting. Only one handler is active at a
// time; the most recent registration wins.
func (m *BackgroundShellManager) SetCompletionHandler(fn CompletionFunc) {
	m.onComplete.Set(fn)
}

// GetBackgroundShellManager returns the singleton background shell manager.
func GetBackgroundShellManager() *BackgroundShellManager {
	backgroundManagerOnce.Do(func() {
		backgroundManager = newBackgroundShellManager()
	})
	return backgroundManager
}

// StartOptions describes a background job. Only Command is required.
type StartOptions struct {
	// WorkingDir is the directory the command runs in.
	WorkingDir string
	// BlockFuncs are the command guards applied to the shell.
	BlockFuncs []BlockFunc
	// Command is the shell command to run.
	Command string
	// Description is a short human-readable label for the job.
	Description string
	// SessionID identifies the conversation that started the job, so its
	// completion can be reported back to the right place. Optional.
	SessionID string
}

// Start creates and starts a new background shell with the given command.
func (m *BackgroundShellManager) Start(ctx context.Context, opts StartOptions) (*BackgroundShell, error) {
	// Check job limit
	if m.shells.Len() >= MaxBackgroundJobs {
		return nil, fmt.Errorf("maximum number of background jobs (%d) reached. Please terminate or wait for some jobs to complete", MaxBackgroundJobs)
	}

	id := fmt.Sprintf("%03X", idCounter.Add(1))

	shell := NewShell(&Options{
		WorkingDir: opts.WorkingDir,
		BlockFuncs: opts.BlockFuncs,
	})

	shellCtx, cancel := context.WithCancel(ctx)

	bgShell := &BackgroundShell{
		ID:          id,
		Command:     opts.Command,
		Description: opts.Description,
		WorkingDir:  opts.WorkingDir,
		SessionID:   opts.SessionID,
		StartedAt:   time.Now(),
		Shell:       shell,
		ctx:         shellCtx,
		cancel:      cancel,
		stdout:      &syncBuffer{},
		stderr:      &syncBuffer{},
		done:        make(chan struct{}),
	}

	// The combined transcript is the single source of truth for reading a
	// job's output at an offset, and it lets a completion report link to
	// the full text instead of inlining it. A job whose log cannot be
	// opened still runs; it just falls back to the in-memory buffers.
	stdoutW, stderrW := io.Writer(bgShell.stdout), io.Writer(bgShell.stderr)
	if logFile, path, err := m.openJobLog(id); err != nil {
		slog.Warn("Failed to create background job log", "id", id, "error", err)
	} else {
		bgShell.log = logFile
		bgShell.LogPath = path
		stdoutW = io.MultiWriter(bgShell.stdout, logFile)
		stderrW = io.MultiWriter(bgShell.stderr, logFile)
	}

	m.shells.Set(id, bgShell)

	go func() {
		// Deferred calls run last-in-first-out, so the handler registered
		// first runs last: the log is flushed and done is closed before
		// anything observes the job, and the job already reads as finished
		// by the time the completion handler sees it.
		defer func() {
			if fn := m.onComplete.Get(); fn != nil {
				fn(bgShell)
			}
		}()
		defer close(bgShell.done)
		defer func() {
			if bgShell.log != nil {
				bgShell.log.Close()
			}
		}()

		err := shell.ExecStream(shellCtx, opts.Command, stdoutW, stderrW)

		bgShell.exitErr = err
		bgShell.completedAt.Store(time.Now().Unix())
	}()

	return bgShell, nil
}

// openJobLog creates the combined transcript file for a job.
func (m *BackgroundShellManager) openJobLog(id string) (*syncFile, string, error) {
	dir := m.logDir.Get()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "job-"+id+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, "", err
	}
	return &syncFile{f: f}, path, nil
}

// Get retrieves a background shell by ID.
func (m *BackgroundShellManager) Get(id string) (*BackgroundShell, bool) {
	return m.shells.Get(id)
}

// Remove removes a background shell from the manager without terminating it.
// This is useful when a shell has already completed and you just want to clean up tracking.
func (m *BackgroundShellManager) Remove(id string) error {
	shell, ok := m.shells.Take(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}
	shell.removeLog()
	return nil
}

// Kill terminates a background shell by ID.
func (m *BackgroundShellManager) Kill(id string) error {
	shell, ok := m.shells.Take(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}

	shell.killed.Store(true)
	shell.cancel()
	<-shell.done
	shell.removeLog()
	return nil
}

// removeLog deletes the job's transcript. Callers do this once the job is no
// longer tracked, so a long session does not leave logs behind for jobs
// nobody can look up anymore.
func (bs *BackgroundShell) removeLog() {
	if bs.LogPath == "" {
		return
	}
	if err := os.Remove(bs.LogPath); err != nil && !os.IsNotExist(err) {
		slog.Debug("Failed to remove background job log", "id", bs.ID, "error", err)
	}
}

// BackgroundShellInfo contains information about a background shell.
type BackgroundShellInfo struct {
	ID          string
	Command     string
	Description string
	WorkingDir  string
	SessionID   string
	StartedAt   time.Time
	Done        bool
	ExitCode    int
	LogPath     string
}

// List returns all background shell IDs.
func (m *BackgroundShellManager) List() []string {
	ids := make([]string, 0, m.shells.Len())
	for id := range m.shells.Seq2() {
		ids = append(ids, id)
	}
	return ids
}

// Jobs returns a snapshot of every tracked job, oldest first. Unlike List it
// carries enough detail to describe a job the caller has never seen before,
// which is what lets a conversation recover job IDs it has forgotten.
func (m *BackgroundShellManager) Jobs() []BackgroundShellInfo {
	infos := make([]BackgroundShellInfo, 0, m.shells.Len())
	for bs := range m.shells.Seq() {
		done := bs.IsDone()
		exitCode := 0
		if done {
			exitCode = ExitCode(bs.exitErr)
		}
		infos = append(infos, BackgroundShellInfo{
			ID:          bs.ID,
			Command:     bs.Command,
			Description: bs.Description,
			WorkingDir:  bs.WorkingDir,
			SessionID:   bs.SessionID,
			StartedAt:   bs.StartedAt,
			Done:        done,
			ExitCode:    exitCode,
			LogPath:     bs.LogPath,
		})
	}
	slices.SortFunc(infos, func(a, b BackgroundShellInfo) int {
		return a.StartedAt.Compare(b.StartedAt)
	})
	return infos
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
			shell.killed.Store(true)
			shell.cancel()
			select {
			case <-shell.done:
			case <-ctx.Done():
			}
			shell.removeLog()
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

// ReadFrom returns the combined stdout/stderr transcript starting at offset,
// along with the offset to pass on the next call. Polling with the returned
// offset yields only what is new, so watching a chatty job does not mean
// re-reading everything it has ever printed.
//
// A negative offset reads from the start. An offset past the end returns an
// empty chunk and the unchanged end position rather than an error, so a
// caller that raced a write can simply try again.
func (bs *BackgroundShell) ReadFrom(offset int64) (chunk string, next int64, err error) {
	if offset < 0 {
		offset = 0
	}

	// Without a transcript on disk there is nothing to seek into, so fall
	// back to the buffers and treat the joined text as the stream.
	if bs.LogPath == "" {
		combined := bs.stdout.String() + bs.stderr.String()
		if offset >= int64(len(combined)) {
			return "", int64(len(combined)), nil
		}
		return combined[offset:], int64(len(combined)), nil
	}

	f, err := os.Open(bs.LogPath)
	if err != nil {
		return "", offset, err
	}
	defer f.Close()

	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return "", offset, err
	}
	if offset >= size {
		return "", size, nil
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return "", offset, err
	}
	buf := make([]byte, size-offset)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", offset, err
	}
	return string(buf[:n]), offset + int64(n), nil
}

// Tail returns the last n lines of the combined transcript and reports
// whether anything was dropped from the front.
func (bs *BackgroundShell) Tail(n int) (text string, truncated bool) {
	full, _, err := bs.ReadFrom(0)
	if err != nil || full == "" {
		return "", false
	}
	lines := strings.Split(strings.TrimRight(full, "\n"), "\n")
	if n <= 0 || len(lines) <= n {
		return strings.Join(lines, "\n"), false
	}
	return strings.Join(lines[len(lines)-n:], "\n"), true
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
