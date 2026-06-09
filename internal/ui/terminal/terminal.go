package terminal

import (
	"context"
	"errors"
	"image/color"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/charmbracelet/x/xpty"
)

// ExitMsg is sent when the terminal process exits.
type ExitMsg struct {
	Err error
}

// OutputMsg signals that there is new output to render.
type OutputMsg struct{}

// DefaultRefreshRate is the default refresh rate for terminal output (24fps).
const DefaultRefreshRate = time.Second / 24

// Config holds configuration for the terminal.
type Config struct {
	// Context controls the terminal lifetime. When cancelled, the
	// process will be killed.
	Context context.Context
	// Cmd is the command to execute.
	Cmd *exec.Cmd
	// RefreshRate is how often to refresh the display.
	RefreshRate time.Duration
}

// Terminal is an embedded terminal that runs a command in a PTY and
// renders it using a virtual terminal emulator.
type Terminal struct {
	mu sync.RWMutex

	ctx   context.Context
	pty   xpty.Pty
	vterm *vt.Emulator
	cmd   *exec.Cmd

	width         int
	height        int
	mouseMode     uv.MouseMode
	cursorVisible bool
	altScreen     bool
	refreshRate   time.Duration

	started bool
	closed  bool
}

// New creates a new Terminal with the given configuration.
func New(cfg Config) *Terminal {
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}

	refreshRate := cfg.RefreshRate
	if refreshRate == 0 {
		refreshRate = DefaultRefreshRate
	}

	// Copy the command so xpty.Start can mutate it (sets SysProcAttr,
	// Stdin/Stdout/Stderr) without affecting the caller's original.
	// Only the fields relevant to PTY execution are copied; Stdin,
	// Stdout, Stderr, ExtraFiles, Cancel, and WaitDelay are
	// intentionally dropped since the PTY owns I/O.
	var cmd *exec.Cmd
	if cfg.Cmd != nil {
		cmd = exec.CommandContext(ctx, cfg.Cmd.Path, cfg.Cmd.Args[1:]...)
		cmd.Dir = cfg.Cmd.Dir
		cmd.Env = cfg.Cmd.Env
		cmd.SysProcAttr = cfg.Cmd.SysProcAttr
	}

	return &Terminal{
		ctx:           ctx,
		cmd:           cmd,
		refreshRate:   refreshRate,
		cursorVisible: true,
	}
}

// Start initializes the PTY and starts the command.
func (t *Terminal) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return errors.New("terminal already closed")
	}
	if t.started {
		return errors.New("terminal already started")
	}
	if t.cmd == nil {
		return errors.New("no command specified")
	}
	if t.width <= 0 || t.height <= 0 {
		return errors.New("invalid dimensions")
	}

	p, err := xpty.NewPty(t.width, t.height)
	if err != nil {
		return err
	}
	t.pty = p

	t.vterm = vt.NewEmulator(t.width, t.height)
	t.vterm.SetDefaultForegroundColor(color.White)
	t.vterm.SetDefaultBackgroundColor(color.Black)
	t.setupCallbacks()

	if err := t.pty.Start(t.cmd); err != nil {
		t.pty.Close()
		t.pty = nil
		t.vterm = nil
		return err
	}

	// Bidirectional I/O between PTY and virtual terminal.
	go func() {
		if _, err := io.Copy(t.pty, t.vterm); err != nil && !isExpectedIOError(err) {
			slog.Debug("Terminal vterm->pty copy error", "error", err)
		}
	}()
	go func() {
		if _, err := io.Copy(t.vterm, t.pty); err != nil && !isExpectedIOError(err) {
			slog.Debug("Terminal pty->vterm copy error", "error", err)
		}
	}()

	t.started = true
	return nil
}

func (t *Terminal) setupCallbacks() {
	t.vterm.SetCallbacks(vt.Callbacks{
		AltScreen: func(active bool) {
			t.altScreen = active
		},
		EnableMode: func(mode ansi.Mode) {
			switch mode {
			case ansi.ModeMouseNormal:
				t.mouseMode = uv.MouseModeClick
			case ansi.ModeMouseButtonEvent:
				t.mouseMode = uv.MouseModeDrag
			case ansi.ModeMouseAnyEvent:
				t.mouseMode = uv.MouseModeMotion
			}
		},
		DisableMode: func(mode ansi.Mode) {
			switch mode {
			case ansi.ModeMouseNormal, ansi.ModeMouseButtonEvent, ansi.ModeMouseAnyEvent:
				t.mouseMode = uv.MouseModeNone
			}
		},
		CursorVisibility: func(visible bool) {
			t.cursorVisible = visible
		},
	})
}

// Resize changes the terminal dimensions.
func (t *Terminal) Resize(width, height int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return errors.New("terminal already closed")
	}

	t.width = width
	t.height = height

	if t.started {
		if t.vterm != nil {
			t.vterm.Resize(width, height)
		}
		if t.pty != nil {
			return t.pty.Resize(width, height)
		}
	}
	return nil
}

// SendText sends text input to the terminal.
func (t *Terminal) SendText(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.vterm != nil && t.started && !t.closed {
		t.vterm.SendText(text)
	}
}

// SendKey sends a key event to the terminal.
func (t *Terminal) SendKey(key tea.KeyPressMsg) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.vterm != nil && t.started && !t.closed {
		t.vterm.SendKey(vt.KeyPressEvent(key))
	}
}

// SendMouse sends a mouse event to the terminal. Coordinates should
// already be adjusted to be relative to the terminal content area.
func (t *Terminal) SendMouse(msg tea.MouseMsg) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.vterm == nil || !t.started || t.closed || t.mouseMode == uv.MouseModeNone {
		return
	}

	switch ev := msg.(type) {
	case tea.MouseClickMsg:
		t.vterm.SendMouse(uv.MouseClickEvent(ev))
	case tea.MouseReleaseMsg:
		t.vterm.SendMouse(uv.MouseReleaseEvent(ev))
	case tea.MouseWheelMsg:
		t.vterm.SendMouse(uv.MouseWheelEvent(ev))
	case tea.MouseMotionMsg:
		// Only forward motion when the program requested it.
		if ev.Button == tea.MouseNone && t.mouseMode != uv.MouseModeMotion {
			return
		}
		if ev.Button != tea.MouseNone && t.mouseMode == uv.MouseModeClick {
			return
		}
		t.vterm.SendMouse(uv.MouseMotionEvent(ev))
	}
}

// SendPaste sends pasted content to the terminal.
func (t *Terminal) SendPaste(content string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.vterm != nil && t.started && !t.closed {
		t.vterm.Paste(content)
	}
}

// Render returns the current terminal content as a styled ANSI string.
func (t *Terminal) Render() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.vterm == nil || !t.started || t.closed {
		return ""
	}
	return t.vterm.Render()
}

// DrawTo draws the terminal content directly into scr, clipped to area.
// It copies cells one-by-one from the emulator so that no styled cell can
// ever land outside area (which is what causes background bleed past a
// dialog border). It reports whether anything was drawn.
func (t *Terminal) DrawTo(scr uv.Screen, area uv.Rectangle) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.vterm == nil || !t.started || t.closed {
		return false
	}

	w := min(area.Dx(), t.vterm.Width())
	h := min(area.Dy(), t.vterm.Height())
	blank := func() *uv.Cell { return &uv.Cell{Content: " ", Width: 1} }
	for y := range h {
		for x := range w {
			cell := t.vterm.CellAt(x, y)
			if cell == nil || cell.Content == "" {
				cell = blank()
			}
			// A wide cell at the last column would spill one column past
			// area into the dialog border. Replace it with a blank.
			if cell.Width > 1 && x+cell.Width > w {
				cell = blank()
			}
			scr.SetCell(area.Min.X+x, area.Min.Y+y, cell)
		}
	}
	return true
}

// CursorPosition returns the current cursor position. Returns (-1, -1)
// if the terminal is not started, closed, or cursor is hidden.
func (t *Terminal) CursorPosition() (x, y int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.vterm == nil || !t.started || t.closed || !t.cursorVisible {
		return -1, -1
	}

	pos := t.vterm.CursorPosition()
	return pos.X, pos.Y
}

// Started reports whether the terminal has been started.
func (t *Terminal) Started() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.started
}

// Closed reports whether the terminal has been closed.
func (t *Terminal) Closed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

// closeTimeout is how long to wait after SIGTERM before sending SIGKILL.
const closeTimeout = 2 * time.Second

// Close gracefully stops the terminal process and cleans up resources.
// It signals the process group asynchronously so it never blocks the
// caller (which would freeze the TUI). Reaping happens in a background
// goroutine that escalates to SIGKILL after closeTimeout.
func (t *Terminal) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	var errs []error

	// Signal and reap asynchronously so we never block the UI.
	cmd := t.cmd
	if cmd != nil {
		go func() {
			_ = killProcessGroup(cmd)

			done := make(chan struct{})
			go func() {
				if cmd.Process != nil {
					_, _ = cmd.Process.Wait()
				}
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(closeTimeout):
				_ = forceKillProcessGroup(cmd)
				<-done
			}
		}()
	}

	// Close PTY immediately so SIGHUP reaches the child as backup.
	if t.pty != nil {
		if err := t.pty.Close(); err != nil {
			errs = append(errs, err)
		}
		t.pty = nil
	}
	if t.vterm != nil {
		if err := t.vterm.Close(); err != nil {
			errs = append(errs, err)
		}
		t.vterm = nil
	}

	return errors.Join(errs...)
}

// WaitCmd returns a tea.Cmd that waits for the process to exit.
func (t *Terminal) WaitCmd() tea.Cmd {
	return func() tea.Msg {
		t.mu.RLock()
		cmd := t.cmd
		ctx := t.ctx
		t.mu.RUnlock()

		if cmd == nil || cmd.Process == nil {
			return ExitMsg{}
		}
		err := xpty.WaitProcess(ctx, cmd)
		return ExitMsg{Err: err}
	}
}

// RefreshCmd returns a tea.Cmd that schedules a periodic refresh.
func (t *Terminal) RefreshCmd() tea.Cmd {
	t.mu.RLock()
	rate := t.refreshRate
	closed := t.closed
	t.mu.RUnlock()

	if closed {
		return nil
	}
	return tea.Tick(rate, func(time.Time) tea.Msg {
		return OutputMsg{}
	})
}

// PrepareCmd creates a command configured for interactive terminal use.
// Unlike the shell package's non-interactive env (which suppresses
// editors, pagers, and forces color), this preserves the user's
// environment and only ensures TERM is set for proper TUI rendering.
// The context controls the command's lifetime.
func PrepareCmd(ctx context.Context, name string, args []string, workDir string, env []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	cmd.SysProcAttr = sysProcAttr()
	baseEnv := os.Environ()
	if !hasEnvKey(baseEnv, "TERM") {
		baseEnv = append(baseEnv, "TERM=xterm-256color")
	}
	if len(env) > 0 {
		cmd.Env = append(baseEnv, env...)
	} else {
		cmd.Env = baseEnv
	}
	return cmd
}

// hasEnvKey reports whether env contains a value for the given key.
func hasEnvKey(env []string, key string) bool {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

// isExpectedIOError reports whether an I/O error from the PTY↔vterm copy
// goroutines is a normal consequence of shutdown rather than a bug. The
// string-matched messages come from os.(*File).checkValid ("file already
// closed") and os.Pipe/pipe.Close ("read/write on closed pipe") in the
// standard library; they lack sentinel errors so string matching is the
// only option.
func isExpectedIOError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		return true
	}
	msg := err.Error()
	return errors.Is(err, context.Canceled) ||
		msg == "file already closed" ||
		msg == "read/write on closed pipe"
}

// IsExpectedExit reports whether an exit error from WaitCmd is a normal
// shutdown (user typed exit, we killed the process, etc.) rather than
// something worth surfacing to the user.
func IsExpectedExit(err error) bool {
	if err == nil {
		return true
	}
	// Signals we send ourselves on Close().
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return true
	}
	if errors.Is(err, os.ErrProcessDone) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "signal:") ||
		strings.Contains(msg, "wait") && strings.Contains(msg, "no child")
}
