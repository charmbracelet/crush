// Package terminal provides a reusable embedded terminal component that runs
// commands in a PTY and renders them using a virtual terminal emulator.
package terminal

import (
	"context"
	"errors"
	"image/color"
	"io"
	"log/slog"
	"os"
	"os/exec"
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
	// Err is the error returned by the process, if any.
	Err error
}

// OutputMsg signals that there is new output to render.
type OutputMsg struct{}

// Config holds configuration for the terminal.
type Config struct {
	// Context is the context for the terminal. When cancelled, the terminal
	// process will be killed.
	Context context.Context
	// Cmd is the command to execute.
	Cmd *exec.Cmd
	// RefreshRate is how often to refresh the display (default: 24fps).
	RefreshRate time.Duration
}

// DefaultRefreshRate is the default refresh rate for terminal output.
const DefaultRefreshRate = time.Second / 24

// Terminal is an embedded terminal that runs a command in a PTY and renders
// it using a virtual terminal emulator.
type Terminal struct {
	mu sync.RWMutex

	ctx   context.Context
	pty   xpty.Pty
	vterm *vt.Emulator
	cmd   *exec.Cmd

	width       int
	height      int
	mouseMode   uv.MouseMode
	refreshRate time.Duration

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

	// Prepare the command with the provided context.
	var cmd *exec.Cmd
	if cfg.Cmd != nil {
		cmd = exec.CommandContext(ctx, cfg.Cmd.Path, cfg.Cmd.Args[1:]...)
		cmd.Dir = cfg.Cmd.Dir
		cmd.Env = cfg.Cmd.Env
		cmd.SysProcAttr = sysProcAttr()
	}

	return &Terminal{
		ctx:         ctx,
		cmd:         cmd,
		refreshRate: refreshRate,
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

	// Create PTY with specified dimensions.
	pty, err := xpty.NewPty(t.width, t.height)
	if err != nil {
		return err
	}
	t.pty = pty

	// Create virtual terminal emulator.
	t.vterm = vt.NewEmulator(t.width, t.height)

	// Set default colors to prevent nil pointer panics when rendering
	// before the terminal has received content with explicit colors.
	t.vterm.SetDefaultForegroundColor(color.White)
	t.vterm.SetDefaultBackgroundColor(color.Black)

	// Set up callbacks to track mouse mode.
	t.setupCallbacks()

	// Start the command in the PTY.
	if err := t.pty.Start(t.cmd); err != nil {
		t.pty.Close()
		t.pty = nil
		t.vterm = nil
		return err
	}

	// Bidirectional I/O between PTY and virtual terminal.
	go func() {
		if _, err := io.Copy(t.pty, t.vterm); err != nil && !isExpectedIOError(err) {
			slog.Debug("terminal vterm->pty copy error", "error", err)
		}
	}()
	go func() {
		if _, err := io.Copy(t.vterm, t.pty); err != nil && !isExpectedIOError(err) {
			slog.Debug("terminal pty->vterm copy error", "error", err)
		}
	}()

	t.started = true
	return nil
}

// setupCallbacks configures vterm callbacks to track mouse mode.
func (t *Terminal) setupCallbacks() {
	t.vterm.SetCallbacks(vt.Callbacks{
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

// SendPaste sends pasted content to the terminal.
func (t *Terminal) SendPaste(content string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.vterm != nil && t.started && !t.closed {
		t.vterm.Paste(content)
	}
}

// SendMouse sends a mouse event to the terminal.
func (t *Terminal) SendMouse(msg tea.MouseMsg) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.vterm == nil || !t.started || t.closed || t.mouseMode == uv.MouseModeNone {
		return
	}

	switch ev := msg.(type) {
	case tea.MouseClickMsg:
		t.vterm.SendMouse(vt.MouseClick(ev))
	case tea.MouseReleaseMsg:
		t.vterm.SendMouse(vt.MouseRelease(ev))
	case tea.MouseWheelMsg:
		t.vterm.SendMouse(vt.MouseWheel(ev))
	case tea.MouseMotionMsg:
		// Check mouse mode for motion events.
		if ev.Button == tea.MouseNone && t.mouseMode != uv.MouseModeMotion {
			return
		}
		if ev.Button != tea.MouseNone && t.mouseMode == uv.MouseModeClick {
			return
		}
		t.vterm.SendMouse(vt.MouseMotion(ev))
	}
}

// Render returns the current terminal content as a string with ANSI styling.
func (t *Terminal) Render() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.vterm == nil || !t.started || t.closed {
		return ""
	}

	return t.vterm.Render()
}

// Started returns whether the terminal has been started.
func (t *Terminal) Started() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.started
}

// Closed returns whether the terminal has been closed.
func (t *Terminal) Closed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

// Close stops the terminal process and cleans up resources.
func (t *Terminal) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	var errs []error

	// Explicitly kill the process if still running.
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}

	// Close PTY.
	if t.pty != nil {
		if err := t.pty.Close(); err != nil {
			errs = append(errs, err)
		}
		t.pty = nil
	}

	// Close virtual terminal.
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

// RefreshCmd returns a tea.Cmd that schedules a refresh.
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

// PrepareCmd creates a command with the given arguments and optional
// working directory. The context parameter controls the command's lifetime.
func PrepareCmd(ctx context.Context, name string, args []string, workDir string, env []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	} else {
		cmd.Env = os.Environ()
	}
	return cmd
}

// isExpectedIOError returns true for errors that are expected when the
// terminal is closing (EOF, closed pipe, etc).
func isExpectedIOError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		return true
	}
	// Check for common close-related error messages.
	msg := err.Error()
	return errors.Is(err, context.Canceled) ||
		msg == "file already closed" ||
		msg == "read/write on closed pipe"
}
