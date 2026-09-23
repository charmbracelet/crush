package shell

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
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
	// Owner is the chat session that started the session through the
	// agent. Empty means the user started it.
	Owner string
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
	owner   string

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

	// inputCh serializes input into a single writer goroutine so callers
	// never block on a wedged child: enqueueing stops with an error once
	// the queue is full instead of blocking on a PTY write forever.
	inputCh   chan func()
	inputStop chan struct{}

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
	focusMu sync.Mutex
	focused bool

	// altScreen tracks whether the child switched to the alternate screen
	// (a full-screen TUI is running).
	altScreen atomic.Bool

	// lastBell is when the child last rang the terminal bell, zero never.
	lastBell atomic.Int64

	// paletteMu guards the theme palette and per-session palette overrides
	// (OSC 4), which OSC 104 uses to restore defaults.
	paletteMu sync.Mutex
	palette   [16]color.Color
	paletteOK bool
	overrides map[int]color.Color

	// cancel tears the child down with the session. It is released once the
	// process is reaped or killed.
	cancel context.CancelFunc
}

// ErrCommandBlocked is returned when a block function rejects the command
// before an interactive session is spawned.
var ErrCommandBlocked = fmt.Errorf("command blocked")

// CheckBlocked parses command and applies blockFuncs to the argv of every
// simple command it contains, recursing into the bodies of shell
// interpreters ("sh -c ...", "bash -lc ...", "eval ...") and the commands
// run by wrappers ("env curl", "nohup curl"): without that, a denied
// command could simply be wrapped. It exists because interactive sessions
// spawn a real shell, so the deny-list that normally runs inside the mvdan
// exec middleware never sees these commands.
//
// The check fails closed: a command that cannot be parsed, a command name
// that only resolves at runtime ("$x", "c$(echo)url"), and a shell reading
// its script from a pipe or redirect are all blocked, since none of them
// can be matched against the deny-list before they run.
func CheckBlocked(command string, blockFuncs []BlockFunc) error {
	if len(blockFuncs) == 0 {
		return nil
	}
	return checkScript(command, blockFuncs, 0)
}

// checkBlockDepth bounds how deep the checker recurses into nested shell
// bodies and wrappers; anything nested deeper is blocked.
const checkBlockDepth = 8

// dynamicArg stands in for a word that only resolves at runtime. It cannot
// appear in a parsed shell literal, so it never matches a deny-list entry.
const dynamicArg = "\x00"

// argWrappers run their arguments as a command, so every suffix of their
// argv is checked as a command of its own.
//
// ponytail: fixed list; programs with their own exec flags (find -exec,
// git -c) are not unwrapped.
var argWrappers = map[string]bool{
	"builtin": true, "caffeinate": true, "command": true, "doas": true,
	"env": true, "exec": true, "nice": true, "nohup": true, "setsid": true,
	"stdbuf": true, "sudo": true, "time": true, "timeout": true,
	"watch": true, "xargs": true,
}

// shellNames are interpreters whose script can be passed with -c or read
// from stdin.
var shellNames = map[string]bool{
	"sh": true, "bash": true, "dash": true, "zsh": true, "ksh": true,
	"ash": true, "fish": true,
}

func blockedErr(reason string) error {
	return fmt.Errorf("%w: %s", ErrCommandBlocked, reason)
}

// checkScript parses a script and walks its commands.
func checkScript(command string, blockFuncs []BlockFunc, depth int) error {
	if depth > checkBlockDepth {
		return blockedErr("command is nested too deeply to check")
	}

	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return blockedErr(fmt.Sprintf("could not parse the command to check it (%v)", err))
	}

	var blockErr error
	syntax.Walk(file, func(node syntax.Node) bool {
		// Returning false only skips this node's children, so keep the
		// first error from being overwritten by a later sibling.
		if blockErr != nil {
			return false
		}
		switch n := node.(type) {
		case *syntax.BinaryCmd:
			if (n.Op == syntax.Pipe || n.Op == syntax.PipeAll) && readsScriptFromStdin(n.Y) {
				blockErr = blockedErr("piping a script into a shell")
			}
		case *syntax.Stmt:
			if hasInputRedirect(n) && readsScriptFromStdin(n) {
				blockErr = blockedErr("redirecting a script into a shell")
			}
		case *syntax.CallExpr:
			blockErr = checkCall(n, blockFuncs, depth)
		}
		return blockErr == nil
	})
	return blockErr
}

// checkCall checks one simple command. Its name must be literal; other
// words that expand at runtime become [dynamicArg].
func checkCall(call *syntax.CallExpr, blockFuncs []BlockFunc, depth int) error {
	if len(call.Args) == 0 {
		return nil
	}
	args := make([]string, len(call.Args))
	for i, word := range call.Args {
		value, ok := wordValue(word)
		if !ok {
			value = dynamicArg
		}
		args[i] = value
	}
	return checkArgs(args, blockFuncs, depth)
}

// checkArgs applies the deny-list to an argv, then to whatever it runs: a
// shell body or a wrapped command.
func checkArgs(args []string, blockFuncs []BlockFunc, depth int) error {
	if depth > checkBlockDepth {
		return blockedErr("command is nested too deeply to check")
	}
	if args[0] == dynamicArg {
		return blockedErr("the command name must be literal, not expanded at runtime")
	}

	base := filepath.Base(args[0])
	named := append([]string{base}, args[1:]...)
	for _, block := range blockFuncs {
		if block(args) || block(named) {
			return blockedErr(args[0])
		}
	}

	if body, ok := shellBody(args); ok {
		if strings.Contains(body, dynamicArg) {
			return blockedErr(base + " runs a script that is expanded at runtime")
		}
		if err := checkScript(body, blockFuncs, depth+1); err != nil {
			return err
		}
	}

	if argWrappers[base] {
		for i := 1; i < len(args); i++ {
			if err := checkArgs(args[i:], blockFuncs, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// shellBody extracts a script body from a shell interpreter or eval
// invocation, so the deny-list can recurse into it.
func shellBody(args []string) (string, bool) {
	base := filepath.Base(args[0])
	if base == "eval" {
		return strings.Join(args[1:], " "), len(args) > 1
	}
	if !shellNames[base] {
		return "", false
	}

	// The script is the argument right after a flag that asks the shell to
	// read it from the command line: -c, and combined forms like -lc or
	// -ec. A "--" between the flag and the script is skipped.
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if len(arg) < 2 || arg[0] != '-' || strings.HasPrefix(arg, "--") {
			continue
		}
		if !strings.Contains(arg[1:], "c") {
			continue
		}
		next := i + 1
		if next < len(args) && args[next] == "--" {
			next++
		}
		if next < len(args) {
			return args[next], true
		}
		return "", false
	}
	return "", false
}

// readsScriptFromStdin reports whether stmt runs a shell that takes its
// script from stdin: no -c body and no script file argument.
func readsScriptFromStdin(stmt *syntax.Stmt) bool {
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	name, ok := wordValue(call.Args[0])
	if !ok || !shellNames[filepath.Base(name)] {
		return false
	}
	for _, word := range call.Args[1:] {
		arg, ok := wordValue(word)
		if !ok || !strings.HasPrefix(arg, "-") || strings.Contains(strings.TrimPrefix(arg, "-"), "c") {
			return false
		}
	}
	return true
}

// hasInputRedirect reports whether stmt feeds its stdin from a file or a
// here-document.
func hasInputRedirect(stmt *syntax.Stmt) bool {
	for _, redir := range stmt.Redirs {
		switch redir.Op {
		case syntax.RdrIn, syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc:
			return true
		}
	}
	return false
}

// wordValue resolves a word to its literal value, or reports false when it
// contains anything that expands at runtime.
func wordValue(word *syntax.Word) (string, bool) {
	if lit := word.Lit(); lit != "" {
		return lit, true
	}

	var b strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			if p.Dollar {
				return "", false
			}
			for _, dp := range p.Parts {
				lit, ok := dp.(*syntax.Lit)
				if !ok {
					return "", false
				}
				b.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	return b.String(), true
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
		owner:        opts.Owner,
		dirty:        make(chan struct{}, 1),
		done:         make(chan struct{}),
		readDone:     make(chan struct{}),
		inputCh:      make(chan func(), interactiveInputQueueSize),
		inputStop:    make(chan struct{}),
		registeredAt: startTime,
		overrides:    make(map[int]color.Color),
	}

	emulator.SetCallbacks(vt.Callbacks{
		Title:            session.setTitle,
		WorkingDirectory: session.setWorkingDir,
		AltScreen:        session.setAltScreen,
		Bell:             session.ringBell,
	})
	registerOscHandlers(session)

	shellPath, args := interactiveShellCommand(opts.Command)
	// The child runs for as long as the session does: the context exists so
	// the process cannot outlive it, and it is released once the child is
	// reaped or killed.
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, shellPath, args...) // #nosec G204 -- the command is the caller's intent.
	cmd.Dir = opts.WorkingDir
	cmd.Env = interactiveEnv(opts.Env)
	session.cancel = cancel

	if err := configureInteractiveProcess(cmd, pty); err != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("interactive session: could not configure process: %w", err)
	}

	session.cmd = cmd
	if err := pty.Start(cmd); err != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("interactive session: could not start %s: %w", shellPath, err)
	}

	// Serialize all input (keys, mouse, paste, palette replies) through one
	// writer goroutine so a wedged child can never block a caller forever.
	go session.inputLoop()

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

// interactiveInputQueueSize bounds the pending input operations; a wedged
// child fills the queue, and further writes fail instead of blocking.
const interactiveInputQueueSize = 128

// inputWriteTimeout bounds how long a caller waits for room in the input
// queue before giving up.
const inputWriteTimeout = 2 * time.Second

// inputLoop applies queued input operations in order. Operations that
// cannot run (closed PTY) fail fast; a single wedged write blocks this
// goroutine and the queue, never the callers.
func (s *InteractiveSession) inputLoop() {
	for {
		select {
		case fn := <-s.inputCh:
			fn()
		case <-s.inputStop:
			return
		}
	}
}

// enqueue schedules an input operation. It fails instead of blocking when
// the child stopped consuming input and the queue is full.
func (s *InteractiveSession) enqueue(fn func()) error {
	return s.submit(fn, inputWriteTimeout)
}

// submit queues fn, waiting up to wait for room in the queue. A zero wait
// never blocks, which keeps the order of calls made from one goroutine.
func (s *InteractiveSession) submit(fn func(), wait time.Duration) error {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return fmt.Errorf("interactive session: session is closed")
	}

	select {
	case s.inputCh <- fn:
		return nil
	default:
	}
	if wait <= 0 {
		return errInputQueueFull
	}
	select {
	case s.inputCh <- fn:
		return nil
	case <-time.After(wait):
		return errInputQueueFull
	}
}

var errInputQueueFull = errors.New("interactive session: the child is not reading input")

// tryEnqueue schedules a best-effort input operation, dropping it when the
// queue is full. Used for replies the child asked for, where silence is
// better than stalling the PTY reader.
func (s *InteractiveSession) tryEnqueue(fn func()) {
	select {
	case s.inputCh <- fn:
	default:
	}
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

	// The child is gone: release the context that would otherwise kill it.
	if s.cancel != nil {
		s.cancel()
	}

	close(s.done)
	s.signalDirty()

	// Nothing can be sent to an exited child: release the PTY and the
	// input goroutines. The emulator stays readable for the transcript.
	_ = s.Close()
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
	return s.enqueue(func() {
		if _, err := s.pty.Write(p); err != nil {
			slog.Debug("Failed to write to interactive session", "error", err)
		}
	})
}

// SendKey sends a semantic keystroke to the child. It goes through the
// emulator's input path, so the encoding matches the modes the child
// enabled (application cursor keys, keypad, mouse), which is what makes
// full-screen programs controllable.
func (s *InteractiveSession) SendKey(key uv.KeyPressEvent) error {
	return s.enqueue(func() {
		s.emu.SendKey(key)
	})
}

// SendMouse sends a mouse event to the child, encoded for the mouse modes
// the child enabled. Programs that never asked for mouse tracking receive
// nothing, just like a real terminal.
func (s *InteractiveSession) SendMouse(event uv.MouseEvent) error {
	return s.enqueue(func() {
		s.emu.SendMouse(event)
	})
}

// Paste sends text as a paste: when the child enabled bracketed paste mode
// the text is wrapped in the paste markers, so multi-line pastes are
// inserted instead of executed line by line.
func (s *InteractiveSession) Paste(text string) error {
	return s.enqueue(func() {
		s.emu.Paste(text)
	})
}

// TrySendKey is [InteractiveSession.SendKey] without blocking: it fails at
// once when the input queue is full. Calls from one goroutine (the UI
// loop) reach the child in order.
func (s *InteractiveSession) TrySendKey(key uv.KeyPressEvent) error {
	return s.submit(func() { s.emu.SendKey(key) }, 0)
}

// TrySendMouse is [InteractiveSession.SendMouse] without blocking.
func (s *InteractiveSession) TrySendMouse(event uv.MouseEvent) error {
	return s.submit(func() { s.emu.SendMouse(event) }, 0)
}

// TryPaste is [InteractiveSession.Paste] without blocking.
func (s *InteractiveSession) TryPaste(text string) error {
	return s.submit(func() { s.emu.Paste(text) }, 0)
}

// Focus tells the child it gained focus, if it asked for focus events.
func (s *InteractiveSession) Focus() {
	s.setFocused(true)
}

// Blur tells the child it lost focus, if it asked for focus events.
func (s *InteractiveSession) Blur() {
	s.setFocused(false)
}

// focused tracks the last focus state sent to the child so repeated calls
// do not re-send the event.
func (s *InteractiveSession) setFocused(focused bool) {
	s.focusMu.Lock()
	had := s.focused
	s.focusMu.Unlock()
	if had == focused {
		return
	}
	s.focusMu.Lock()
	s.focused = focused
	s.focusMu.Unlock()

	s.tryEnqueue(func() {
		// Focus and Blur are not wrapped by [vt.SafeEmulator], so they would
		// race with the read loop's writes to the emulator's modes. The
		// session mutex serializes them like SendKey and Paste are by the
		// emulator's own lock.
		s.emuMu.Lock()
		defer s.emuMu.Unlock()
		if focused {
			s.emu.Focus()
		} else {
			s.emu.Blur()
		}
	})
}

// SetPalette records the theme palette backing the emulator's default ANSI
// colors, which OSC 104 uses to restore defaults after a program changes
// them.
func (s *InteractiveSession) SetPalette(palette [16]color.Color) {
	s.paletteMu.Lock()
	defer s.paletteMu.Unlock()
	s.palette = palette
	s.paletteOK = true
}

// effectiveColor returns the color an index currently resolves to: a
// program-set override, else the theme palette, else the plain palette.
func (s *InteractiveSession) effectiveColor(i int) color.Color {
	s.paletteMu.Lock()
	defer s.paletteMu.Unlock()
	return s.effectiveColorLocked(i)
}

// effectiveColorLocked is effectiveColor for callers that already hold the
// palette mutex.
func (s *InteractiveSession) effectiveColorLocked(i int) color.Color {
	if c, ok := s.overrides[i]; ok {
		return c
	}
	if i < 16 && s.paletteOK {
		return s.palette[i]
	}
	return ansi.IndexedColor(i)
}

// setOverride records a program's palette change and schedules it on the
// emulator. The emulator update is deferred: OSC handlers run inside the
// emulator's write lock, so touching it here would deadlock.
func (s *InteractiveSession) setOverride(i int, c color.Color) {
	s.paletteMu.Lock()
	s.overrides[i] = c
	s.paletteMu.Unlock()

	s.tryEnqueue(func() {
		s.emu.SetIndexedColor(i, c)
	})
}

// clearOverride drops a program's palette change and restores the default.
func (s *InteractiveSession) clearOverride(i int) {
	s.paletteMu.Lock()
	delete(s.overrides, i)
	c := s.effectiveColorLocked(i)
	s.paletteMu.Unlock()

	s.tryEnqueue(func() {
		s.emu.SetIndexedColor(i, c)
	})
}

// clearOverrides drops every program palette change and restores defaults.
func (s *InteractiveSession) clearOverrides() {
	s.paletteMu.Lock()
	overrides := s.overrides
	s.overrides = make(map[int]color.Color)
	palette := s.palette
	paletteOK := s.paletteOK
	s.paletteMu.Unlock()

	if len(overrides) == 0 {
		return
	}
	s.tryEnqueue(func() {
		for i := range overrides {
			if i < 16 && paletteOK {
				s.emu.SetIndexedColor(i, palette[i])
			} else {
				s.emu.SetIndexedColor(i, ansi.IndexedColor(i))
			}
		}
	})
}

// InAltScreen reports whether the child switched to the alternate screen,
// which means a full-screen TUI is drawing.
func (s *InteractiveSession) InAltScreen() bool {
	return s.altScreen.Load()
}

// LastBell is when the child last rang the terminal bell, zero never.
func (s *InteractiveSession) LastBell() time.Time {
	n := s.lastBell.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// setAltScreen tracks alternate-screen changes and repaints the panel,
// whose header shows whether a full-screen TUI is running.
func (s *InteractiveSession) setAltScreen(on bool) {
	if s.altScreen.Swap(on) != on {
		s.signalDirty()
	}
}

// ringBell records a terminal bell and repaints so the UI can surface it.
func (s *InteractiveSession) ringBell() {
	s.lastBell.Store(time.Now().UnixNano())
	s.signalDirty()
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
	err := killInteractiveProcess(s.cmd)
	if s.cancel != nil {
		s.cancel()
	}
	return err
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
	// Stop applying queued input. The channel is never closed so a sender
	// that raced past the closed check cannot panic; its operation is just
	// dropped.
	close(s.inputStop)
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

// Owner returns the chat session that started this session through the
// agent, or the empty string for sessions the user started.
func (s *InteractiveSession) Owner() string { return s.owner }

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
