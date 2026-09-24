package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// TerminalID is the identifier for the interactive terminal dialog.
const TerminalID = "terminal"

// TerminalOutputMsg signals that the embedded terminal changed and should
// repaint.
type TerminalOutputMsg struct{}

// TerminalExitMsg signals that the embedded terminal's process exited.
type TerminalExitMsg struct{}

// Terminal dialog layout constants.
const (
	// terminalHeaderLines is the number of lines the header occupies inside
	// the frame.
	terminalHeaderLines = 1
	// terminalMinCols/terminalMinRows is the smallest area where the
	// dialog still renders its content instead of a "too small" notice.
	terminalMinCols = shell.MinInteractiveCols + 2
	terminalMinRows = shell.MinInteractiveRows + 3
)

// TerminalDialogOptions configures a terminal dialog.
type TerminalDialogOptions struct {
	// Command is displayed in the header; for a bare interactive shell
	// pass the user's shell path.
	Command string
	// AgentStarted marks sessions the agent opened, labeled in the header
	// so the user can tell who created the session.
	AgentStarted bool
	// AgentDriven marks sessions the agent owns and drives itself: user
	// keystrokes and mouse events are refused so the panel is a read-only
	// view of the agent's work.
	AgentDriven bool
}

// TerminalDialog embeds a live PTY session and forwards all input to it.
// The model owns the session lifecycle; the dialog only forwards input and
// paints the emulator.
type TerminalDialog struct {
	com          *common.Common
	session      *shell.InteractiveSession
	command      string
	agentStarted bool
	agentDriven  bool
	terminated   bool
	fullscreen   bool
	contentRect  uv.Rectangle
	closeKey     key.Binding
	fullKey      key.Binding
}

var _ Dialog = (*TerminalDialog)(nil)

// NewTerminalDialog creates an interactive terminal dialog around an
// already-running session.
func NewTerminalDialog(com *common.Common, session *shell.InteractiveSession, opts TerminalDialogOptions) *TerminalDialog {
	t := &TerminalDialog{
		com:          com,
		session:      session,
		command:      opts.Command,
		agentStarted: opts.AgentStarted,
		agentDriven:  opts.AgentDriven,
	}
	t.closeKey = key.NewBinding(
		key.WithKeys("ctrl+q"),
		key.WithHelp("ctrl+q", "quit"),
	)
	t.fullKey = key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", "fullscreen"),
	)
	t.applyTheme()
	return t
}

// SetFullscreen records whether the terminal currently covers the whole
// window, so the header can show the matching toggle hint.
func (t *TerminalDialog) SetFullscreen(fullscreen bool) {
	t.fullscreen = fullscreen
}

// applyTheme points the emulator at the theme palette so child output is
// legible on the themed background instead of the host terminal's.
func (t *TerminalDialog) applyTheme() {
	sty := t.com.Styles
	emu := t.session.Emulator()
	emu.SetForegroundColor(sty.Terminal.Fg)
	emu.SetBackgroundColor(sty.Terminal.Bg)
	emu.SetCursorColor(sty.Terminal.Cursor)
	for i, c := range sty.ANSI {
		emu.SetIndexedColor(i, c)
	}
}

// ID implements [Dialog].
func (*TerminalDialog) ID() string { return TerminalID }

// Session returns the underlying interactive session.
func (t *TerminalDialog) Session() *shell.InteractiveSession { return t.session }

// Command returns the command displayed in the header.
func (t *TerminalDialog) Command() string { return t.command }

// Keys returns the terminal's own key bindings so the host UI can match
// them and show them in its hints.
func (t *TerminalDialog) Keys() (close, fullscreen key.Binding) {
	return t.closeKey, t.fullKey
}

// HandleMsg implements [Dialog].
func (t *TerminalDialog) HandleMsg(msg tea.Msg) Action {
	emu := t.session.Emulator()

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, t.closeKey) {
			t.terminated = true
			return ActionTerminalKill{}
		}
		if key.Matches(msg, t.fullKey) {
			return ActionTerminalFullscreen{}
		}
		// Agent-driven sessions are read-only for the user: their keys must
		// not reach a command the agent is driving. The agent's write tool
		// bypasses the dialog entirely.
		if t.agentDriven {
			return nil
		}
		return ActionTerminalInput{Apply: func() {
			emu.SendKey(uv.KeyPressEvent(uv.Key(msg)))
		}}

	case tea.PasteMsg:
		if t.agentDriven {
			return nil
		}
		content := msg.Content
		return ActionTerminalInput{Apply: func() {
			emu.Paste(content)
		}}

	case tea.MouseClickMsg:
		if t.agentDriven {
			return nil
		}
		m, ok := t.translateMouse(uv.Mouse(msg))
		if !ok {
			return nil
		}
		return ActionTerminalInput{Apply: func() {
			emu.SendMouse(uv.MouseClickEvent(m))
		}}

	case tea.MouseReleaseMsg:
		if t.agentDriven {
			return nil
		}
		m, ok := t.translateMouse(uv.Mouse(msg))
		if !ok {
			return nil
		}
		return ActionTerminalInput{Apply: func() {
			emu.SendMouse(uv.MouseReleaseEvent(m))
		}}

	case tea.MouseWheelMsg:
		if t.agentDriven {
			return nil
		}
		m, ok := t.translateMouse(uv.Mouse(msg))
		if !ok {
			return nil
		}
		return ActionTerminalInput{Apply: func() {
			emu.SendMouse(uv.MouseWheelEvent(m))
		}}

	case tea.MouseMotionMsg:
		if t.agentDriven {
			return nil
		}
		m, ok := t.translateMouse(uv.Mouse(msg))
		if !ok {
			return nil
		}
		return ActionTerminalInput{Apply: func() {
			emu.SendMouse(uv.MouseMotionEvent(m))
		}}

	case TerminalOutputMsg:
		// Returning nil is enough: the message already triggered a
		// repaint with fresh emulator content.
		return nil

	case TerminalExitMsg:
		return ActionTerminalComplete{Result: t.result()}
	}

	return nil
}

// translateMouse maps a window-level mouse event onto the emulator's
// content area, dropping events outside it.
func (t *TerminalDialog) translateMouse(m uv.Mouse) (uv.Mouse, bool) {
	if !uv.Pos(m.X, m.Y).In(t.contentRect) {
		return uv.Mouse{}, false
	}
	m.X -= t.contentRect.Min.X
	m.Y -= t.contentRect.Min.Y
	return m, true
}

// TerminalResult is the final transcript of an interactive session.
type TerminalResult struct {
	Output     string
	ExitCode   int
	WorkingDir string
	Terminated bool
}

// result builds the outcome for the finished session.
func (t *TerminalDialog) result() TerminalResult {
	return TerminalResult{
		Output:     t.session.CaptureText(),
		ExitCode:   t.session.ExitCode(),
		WorkingDir: t.session.WorkingDir(),
		Terminated: t.terminated,
	}
}

// Draw implements [Dialog]. The dialog takes over the full window: a
// framed box with a header row and the live emulator below.
func (t *TerminalDialog) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	sty := t.com.Styles

	if area.Dx() < terminalMinCols || area.Dy() < terminalMinRows {
		view := sty.Terminal.TooSmall.Render("terminal too small")
		DrawCenter(scr, area, view)
		return nil
	}

	contentWidth := area.Dx() - 2
	contentHeight := area.Dy() - 2 - terminalHeaderLines

	// The emulator must exactly fill the content area; resizing it here
	// keeps PTY, emulator, and layout in sync without an extra message
	// round-trip.
	if cols, rows := t.session.Size(); cols != contentWidth || rows != contentHeight {
		t.session.Resize(contentWidth, contentHeight)
	}

	header := t.renderHeader(contentWidth)

	box := sty.Terminal.Border.
		Width(area.Dx()).
		Height(area.Dy()).
		Render(header)
	uv.NewStyledString(box).Draw(scr, area)

	t.contentRect = uv.Rect(area.Min.X+1, area.Min.Y+1+terminalHeaderLines, contentWidth, contentHeight)
	t.session.Emulator().Draw(scr, t.contentRect)

	return t.cursor()
}

// renderHeader builds the one-line header: "$ command" on the left, then
// the live status chip and the keybinding hints on the right.
func (t *TerminalDialog) renderHeader(width int) string {
	sty := t.com.Styles

	fullHint := t.fullKey.Help().Key + " " + t.fullKey.Help().Desc
	if t.fullscreen {
		fullHint = t.fullKey.Help().Key + " docked"
	}
	hintText := t.closeKey.Help().Key + " " + t.closeKey.Help().Desc + " · " + fullHint
	if t.agentDriven {
		// The user watches but does not type here; say so plainly.
		hintText = "read-only · " + hintText
	}
	hint := sty.Terminal.Hint.Render(hintText)

	status := t.renderStatus()
	rightWidth := lipgloss.Width(hint) + lipgloss.Width(status)

	commandWidth := max(width-rightWidth-2, 1)
	command := ansi.Truncate(t.command, commandWidth, "…")
	left := sty.Terminal.Header.Render("$ " + command)

	pad := max(width-lipgloss.Width(left)-rightWidth, 0)
	// The status chip and hints can outgrow a narrow panel; keep the header
	// to a single row so the frame height stays predictable.
	return ansi.Truncate(left+strings.Repeat(" ", pad)+status+hint, width, "…")
}

// renderStatus builds the live status chip shown before the hint: whether
// the command is still running, and whether the agent opened this session
// or the user did. The emulator below shows the activity itself; the chip
// makes it clear at a glance who is driving and that work is happening.
func (t *TerminalDialog) renderStatus() string {
	sty := t.com.Styles

	state := "● running"
	if t.session.Exited() {
		state = fmt.Sprintf("exited %d", t.session.ExitCode())
	}
	status := sty.Terminal.Status.Render(state)
	if t.agentStarted {
		status = sty.Terminal.Agent.Render("agent") + " " + status
	}
	return status + " "
}

// cursor returns the emulator cursor, offset by the content area. It
// returns nil when the child hid its cursor: TUIs hide it constantly, and
// showing one anyway puts a phantom cursor somewhere on screen.
func (t *TerminalDialog) cursor() *tea.Cursor {
	emu := t.session.Emulator()
	if emu.CursorHidden() {
		return nil
	}

	style, blink := emu.CursorStyle()
	pos := emu.CursorPosition()

	return &tea.Cursor{
		Position: tea.Position{X: t.contentRect.Min.X + pos.X, Y: t.contentRect.Min.Y + pos.Y},
		Color:    t.com.Styles.Terminal.Cursor,
		Shape:    cursorShape(style),
		Blink:    blink,
	}
}

// cursorShape maps an emulator cursor style onto Bubble Tea's.
func cursorShape(style vt.CursorStyle) tea.CursorShape {
	switch style {
	case vt.CursorUnderline:
		return tea.CursorUnderline
	case vt.CursorBar:
		return tea.CursorBar
	default:
		return tea.CursorBlock
	}
}
