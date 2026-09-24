package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/charmbracelet/x/xpty"
	"mvdan.cc/sh/v3/syntax"
)

// Interactive terminal defaults.
const (
	// DefaultInteractiveScrollback is the number of scrolled-off lines kept
	// for the final transcript handed back to the caller.
	DefaultInteractiveScrollback = 2000
	// MinInteractiveCols and MinInteractiveRows clamp the emulated terminal
	// so a tiny window still yields a usable screen.
	MinInteractiveCols = 20
	MinInteractiveRows = 5
	// DefaultInteractiveCols and DefaultInteractiveRows size a session
	// spawned before the UI has attached; the dialog resizes it to the
	// real window on the first frame.
	DefaultInteractiveCols = 80
	DefaultInteractiveRows = 24
	// interactiveReadBufferSize is the per-read buffer for PTY output.
	interactiveReadBufferSize = 32 * 1024
)

// interactiveEnvVars make a PTY-backed command behave like it is running in
// a real terminal: programs that honor FORCE_COLOR/CLICOLOR_FORCE emit color
// even when they only check those variables, and TERM describes what the
// embedded emulator actually implements.
var interactiveEnvVars = []string{
	"TERM=xterm-256color",
	"COLORTERM=truecolor",
	"CLICOLOR_FORCE=1",
	"FORCE_COLOR=1",
}

// InteractiveSessionOptions configures a new [InteractiveSession].
type InteractiveSessionOptions struct {
	// Command is the shell source to run. An empty command starts an
	// interactive shell instead.
	Command string
	// WorkingDir is the directory the session starts in.
	WorkingDir string
	// Env is the full environment. nil inherits the Crush process
	// environment (minus herdr pane ownership variables).
	Env []string
	// BlockFuncs is an optional deny-list applied to the parsed command
	// before the session is spawned. nil disables blocking entirely.
	BlockFuncs []BlockFunc
	// Cols and Rows size the PTY and its emulator.
	Cols, Rows int
	// ScrollbackSize overrides [DefaultInteractiveScrollback].
	ScrollbackSize int
}

// InteractiveSession runs a command attached to a real PTY while mirroring
// its output into an in-process terminal emulator. Callers render the
// emulator, forward input through its input pipe, and read [CaptureText]
// once the session ends.
//
// This intentionally bypasses the mvdan interpreter used by [Shell] and
// [Run]: those run with piped stdio and a detached session (see
// [isolateProcess]), which is exactly what makes prompts and full-screen
// TUIs impossible. Here the child is a session leader with the PTY slave as
// its controlling terminal.
type InteractiveSession struct {
	emu     *vt.SafeEmulator
	pty     xpty.Pty
	cmd     *exec.Cmd
	command string

	// id is assigned by the manager; empty for sessions that never
	// registered.
	id string

	// registeredAt and completedAt (Unix minutes) drive retention.
	registeredAt int64
	completedAt  atomic.Int64

	// input is the emulator's input pipe. Everything the emulator encodes
	// (keys, mouse, paste) is read from it and written to the PTY. Closing
	// it ends the forwarding goroutine.
	input io.Closer

	dirty    chan struct{}
	done     chan struct{}
	readDone chan struct{}

	// emuMu serializes mutations of the emulator with text extraction.
	// [vt.SafeEmulator] locks its rendering and write paths, but not
	// String/Scrollback, so the session guards those itself.
	emuMu sync.RWMutex

	mu      sync.Mutex
	exitErr error
	closed  bool

	titleMu sync.Mutex
	title   string
	cwdMu   sync.Mutex
	cwd     string
}

// ErrCommandBlocked is returned when a block function rejects the command
// before an interactive session is spawned.
var ErrCommandBlocked = fmt.Errorf("command blocked")

// CheckBlocked parses command and applies blockFuncs to the literal argv of
// every simple command it contains. It exists because interactive sessions
// spawn a real shell, so the deny-list that normally runs inside the mvdan
// exec middleware never sees these commands.
//
// Commands that cannot be parsed are passed through: the spawned shell
// reports the syntax error itself, and an unparsable command cannot be
// matched against the deny-list anyway.
func CheckBlocked(command string, blockFuncs []BlockFunc) error {
	if len(blockFuncs) == 0 {
		return nil
	}

	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return nil
	}

	var blocked string
	syntax.Walk(file, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		args, ok := literalArgs(call)
		if !ok {
			return true
		}
		for _, block := range blockFuncs {
			if block(args) {
				blocked = args[0]
				return false
			}
		}
		return true
	})

	if blocked != "" {
		return fmt.Errorf("%w: %s", ErrCommandBlocked, blocked)
	}
	return nil
}

// literalArgs returns the literal argv of a call expression. It reports
// false when any word requires expansion (a variable, glob, or command
// substitution), since those cannot be resolved before execution.
func literalArgs(call *syntax.CallExpr) ([]string, bool) {
	if len(call.Args) == 0 {
		return nil, false
	}
	args := make([]string, 0, len(call.Args))
	for _, word := range call.Args {
		lit := word.Lit()
		if lit == "" {
			return nil, false
		}
		args = append(args, lit)
	}
	return args, true
}

// NewInteractiveSession spawns command on a new PTY with a terminal
// emulator attached. The returned session is already running.
func NewInteractiveSession(opts InteractiveSessionOptions) (*InteractiveSession, error) {
	if opts.WorkingDir == "" {
		return nil, fmt.Errorf("interactive session: working directory is required")
	}
	if err := CheckBlocked(opts.Command, opts.BlockFuncs); err != nil {
		return nil, err
	}

	cols := max(opts.Cols, MinInteractiveCols)
	rows := max(opts.Rows, MinInteractiveRows)

	emulator := vt.NewSafeEmulator(cols, rows)
	emulator.SetScrollbackSize(cmpOr(opts.ScrollbackSize, DefaultInteractiveScrollback))

	pty, err := xpty.NewPty(cols, rows)
	if err != nil {
		return nil, fmt.Errorf("interactive session: could not allocate pty: %w", err)
	}

	// The emulator's input pipe is an io.PipeWriter, which the interface
	// only exposes as an io.Writer. It is the handle that unblocks the
	// input forwarding goroutine, so keep a closable reference.
	input, _ := emulator.InputPipe().(io.Closer)

	startTime := time.Now().Unix() / 60

	session := &InteractiveSession{
		emu:          emulator,
		pty:          pty,
		input:        input,
		command:      opts.Command,
		dirty:        make(chan struct{}, 1),
		done:         make(chan struct{}),
		readDone:     make(chan struct{}),
		registeredAt: startTime,
	}

	emulator.SetCallbacks(vt.Callbacks{
		Title:            session.setTitle,
		WorkingDirectory: session.setWorkingDir,
	})

	shellPath, args := interactiveShellCommand(opts.Command)
	cmd := exec.Command(shellPath, args...) // #nosec G204 -- the command is the caller's intent.
	cmd.Dir = opts.WorkingDir
	cmd.Env = interactiveEnv(opts.Env)

	if err := configureInteractiveProcess(cmd, pty); err != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("interactive session: could not configure process: %w", err)
	}

	session.cmd = cmd
	if err := pty.Start(cmd); err != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("interactive session: could not start %s: %w", shellPath, err)
	}

	// Forward everything the emulator encodes (keys, mouse, paste) to the
	// PTY. The emulator exposes its input as a reader, so this is the
	// inverse of the output loop below.
	go func() {
		_, _ = io.Copy(pty, emulator)
	}()

	go session.readLoop()
	go session.waitLoop()

	return session, nil
}

// readLoop mirrors PTY output into the emulator and signals the UI. It
// closes when the PTY master stops producing output, which normally
// happens once every process holding the slave has exited.
func (s *InteractiveSession) readLoop() {
	defer close(s.readDone)

	buf := make([]byte, interactiveReadBufferSize)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			s.emuMu.Lock()
			_, _ = s.emu.Write(buf[:n])
			s.emuMu.Unlock()
			s.signalDirty()
		}
		if err != nil {
			return
		}
	}
}

// readDrainTimeout bounds how long waitLoop waits for the reader to drain
// remaining output after the process exits. A grandchild that inherited
// the slave keeps the master readable forever, so the wait is not unbounded.
const readDrainTimeout = 250 * time.Millisecond

// waitLoop reaps the child, lets the reader drain any remaining output,
// and records the exit error.
func (s *InteractiveSession) waitLoop() {
	err := xpty.WaitProcess(context.Background(), s.cmd)

	// The process is gone, but buffered output may still be in flight;
	// give the reader a moment so the final screen is complete.
	select {
	case <-s.readDone:
	case <-time.After(readDrainTimeout):
	}

	s.mu.Lock()
	s.exitErr = err
	s.mu.Unlock()
	s.completedAt.Store(time.Now().Unix() / 60)
	syncWorkDir(s)

	close(s.done)
	s.signalDirty()
}

// signalDirty wakes the UI without blocking the PTY reader: renderable
// output is coalesced into a single pending notification.
func (s *InteractiveSession) signalDirty() {
	select {
	case s.dirty <- struct{}{}:
	default:
	}
}

func (s *InteractiveSession) setTitle(title string) {
	s.titleMu.Lock()
	defer s.titleMu.Unlock()
	s.title = title
}

func (s *InteractiveSession) setWorkingDir(dir string) {
	s.cwdMu.Lock()
	defer s.cwdMu.Unlock()
	s.cwd = dir
}

// Title returns the title last reported by the child, if any.
func (s *InteractiveSession) Title() string {
	s.titleMu.Lock()
	defer s.titleMu.Unlock()
	return s.title
}

// WorkingDir returns the child's current working directory when it has
// reported one (via OSC 7), falling back to the configured one.
func (s *InteractiveSession) WorkingDir() string {
	s.cwdMu.Lock()
	defer s.cwdMu.Unlock()
	if s.cwd == "" {
		return s.cmd.Dir
	}
	return s.cwd
}

// syncWorkDir persists an OSC 7 directory change for later queries.
func syncWorkDir(s *InteractiveSession) {
	s.cwdMu.Lock()
	defer s.cwdMu.Unlock()
	if s.cwd == "" {
		s.cwd = s.cmd.Dir
	}
}

// Emulator returns the terminal emulator backing this session. It is safe
// for concurrent use.
func (s *InteractiveSession) Emulator() *vt.SafeEmulator { return s.emu }

// Dirty returns a coalesced signal that fires when the emulator changed.
func (s *InteractiveSession) Dirty() <-chan struct{} { return s.dirty }

// Done is closed once the child has exited.
func (s *InteractiveSession) Done() <-chan struct{} { return s.done }

// Exited reports whether the child has exited.
func (s *InteractiveSession) Exited() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// ExitErr returns the error the child exited with, or nil on success. It is
// only meaningful after [InteractiveSession.Done] is closed.
func (s *InteractiveSession) ExitErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitErr
}

// ExitCode returns the child's exit status using shell conventions.
func (s *InteractiveSession) ExitCode() int {
	return InteractiveExitCode(s.ExitErr())
}

// InteractiveExitCode maps an exit error to a conventional exit status.
func InteractiveExitCode(err error) int {
	if err == nil {
		return 0
	}
	if code, ok := signaledExitCode(err); ok {
		return code
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
	}
	return 1
}

// Write sends raw bytes to the child, bypassing the emulator's encoding.
func (s *InteractiveSession) Write(p []byte) error {
	if _, err := s.pty.Write(p); err != nil {
		return fmt.Errorf("interactive session: write failed: %w", err)
	}
	return nil
}

// SendKey sends a semantic keystroke to the child. It goes through the
// emulator's input path, so the encoding matches the modes the child
// enabled (application cursor keys, keypad, mouse), which is what makes
// full-screen programs controllable.
func (s *InteractiveSession) SendKey(key uv.KeyPressEvent) error {
	if s.Exited() {
		return fmt.Errorf("interactive session: session %s has exited", s.ID())
	}
	s.emu.SendKey(key)
	return nil
}

// SendMouse sends a mouse event to the child, encoded for the mouse modes
// the child enabled. Programs that never asked for mouse tracking receive
// nothing, just like a real terminal.
func (s *InteractiveSession) SendMouse(event uv.MouseEvent) error {
	if s.Exited() {
		return fmt.Errorf("interactive session: session %s has exited", s.ID())
	}
	s.emu.SendMouse(event)
	return nil
}

// Resize resizes both the emulator and the underlying PTY.
func (s *InteractiveSession) Resize(cols, rows int) {
	cols = max(cols, MinInteractiveCols)
	rows = max(rows, MinInteractiveRows)

	s.emuMu.Lock()
	s.emu.Resize(cols, rows)
	s.emuMu.Unlock()

	if err := s.pty.Resize(cols, rows); err != nil {
		slog.Debug("Failed to resize interactive session", "error", err)
	}
}

// Size returns the current PTY dimensions.
func (s *InteractiveSession) Size() (cols, rows int) {
	s.emuMu.RLock()
	defer s.emuMu.RUnlock()
	cols, rows = s.emu.Width(), s.emu.Height()
	return cols, rows
}

// Kill terminates the child and its descendants.
func (s *InteractiveSession) Kill() error {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return nil
	}
	return killInteractiveProcess(s.cmd)
}

// Close releases the PTY and unblocks the emulator's input pipe. The child
// is not terminated; call [InteractiveSession.Kill] first when needed.
//
// The emulator's own Close is deliberately not used: it flips a flag that
// the (unlocked) input reader checks, which the race detector flags. Closing
// the pipe writer ends the reader instead, without touching that state.
func (s *InteractiveSession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	if s.input != nil {
		_ = s.input.Close()
	}
	return s.pty.Close()
}

// CaptureText renders the session for a reader that never saw the screen:
// every line that scrolled off, followed by the final visible screen. Raw
// ANSI bytes are deliberately not used because a full-screen app's redraw
// stream repeats itself and would read as garbage.
func (s *InteractiveSession) CaptureText() string {
	s.emuMu.RLock()
	defer s.emuMu.RUnlock()

	var b strings.Builder

	if scrollback := s.emu.Scrollback(); scrollback != nil {
		for i := range scrollback.Len() {
			line := scrollback.Line(i)
			if line == nil {
				continue
			}
			b.WriteString(strings.TrimRight(line.String(), " \t"))
			b.WriteByte('\n')
		}
	}

	b.WriteString(strings.TrimRight(s.emu.String(), " \n"))
	return strings.TrimRight(b.String(), " \n")
}

// ID returns the session's manager-assigned ID, or the empty string for
// sessions that never registered with a manager.
func (s *InteractiveSession) ID() string { return s.id }

// Command returns the command the session is running. An empty string
// means the session is a bare interactive shell.
func (s *InteractiveSession) Command() string { return s.command }

// completedAgeMinutes is how many minutes have passed since the process
// exited; 0 while it is still running.
func (s *InteractiveSession) completedAgeMinutes() int64 {
	completed := s.completedAt.Load()
	if completed == 0 {
		return 0
	}
	return time.Now().Unix()/60 - completed
}

// ScreenText returns the emulator's current visible screen as plain text.
func (s *InteractiveSession) ScreenText() string {
	s.emuMu.RLock()
	defer s.emuMu.RUnlock()
	return strings.TrimRight(s.emu.String(), " \n")
}

// ScrollbackText returns every line that scrolled off the visible screen.
func (s *InteractiveSession) ScrollbackText() string {
	s.emuMu.RLock()
	defer s.emuMu.RUnlock()

	var b strings.Builder
	if scrollback := s.emu.Scrollback(); scrollback != nil {
		for i := range scrollback.Len() {
			line := scrollback.Line(i)
			if line == nil {
				continue
			}
			b.WriteString(strings.TrimRight(line.String(), " \t"))
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), " \n")
}

// Cursor returns the emulator cursor position and whether it is hidden.
func (s *InteractiveSession) Cursor() (x, y int, hidden bool) {
	pos := s.emu.CursorPosition()
	return pos.X, pos.Y, s.emu.CursorHidden()
}

// interactiveShellCommand resolves the shell that will host the command. An
// empty command starts an interactive shell so callers can hand the user a
// prompt.
func interactiveShellCommand(command string) (string, []string) {
	return platformInteractiveShell(command)
}

// interactiveEnv builds the environment for an interactive session.
func interactiveEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	env = withoutHerdrEnv(env)
	env = withEnvOverrides(env, CrushEnvMarkers())
	return withEnvOverrides(env, interactiveEnvVars)
}

// withEnvOverrides returns env with overrides forced in, replacing any
// existing values for the same keys. The returned slice is a new allocation
// safe to use concurrently with the input.
func withEnvOverrides(env, overrides []string) []string {
	overrideKeys := make(map[string]bool, len(overrides))
	for _, kv := range overrides {
		if key, _, ok := strings.Cut(kv, "="); ok {
			overrideKeys[key] = true
		}
	}

	result := make([]string, 0, len(env)+len(overrides))
	for _, e := range env {
		if key, _, ok := strings.Cut(e, "="); ok && overrideKeys[key] {
			continue
		}
		result = append(result, e)
	}
	return append(result, overrides...)
}

// cmpOr returns fallback when value is the zero value.
func cmpOr(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
