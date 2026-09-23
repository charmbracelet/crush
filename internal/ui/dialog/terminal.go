package dialog

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
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

// TerminalDialog embeds a live PTY session and forwards all input to it.
// The model owns the session lifecycle; the dialog only forwards input and
// paints the emulator.
type TerminalDialog struct {
	com         *common.Common
	session     *shell.InteractiveSession
	command     string
	terminated  bool
	contentRect uv.Rectangle
	closeKey    key.Binding
}

var _ Dialog = (*TerminalDialog)(nil)

// NewTerminalDialog creates an interactive terminal dialog around an
// already-running session. command is displayed in the header; for a bare
// interactive shell pass the user's shell path.
func NewTerminalDialog(com *common.Common, session *shell.InteractiveSession, command string) *TerminalDialog {
	t := &TerminalDialog{
		com:     com,
		session: session,
		command: command,
	}
	t.closeKey = key.NewBinding(
		key.WithKeys("ctrl+q"),
		key.WithHelp("ctrl+q", "close"),
	)
	t.applyTheme()
	return t
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

// HandleMsg implements [Dialog].
func (t *TerminalDialog) HandleMsg(msg tea.Msg) Action {
	emu := t.session.Emulator()

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, t.closeKey) {
			t.terminated = true
			return ActionTerminalKill{}
		}
		return ActionTerminalInput{Apply: func() {
			emu.SendKey(uv.KeyPressEvent(uv.Key(msg)))
		}}

	case tea.PasteMsg:
		content := msg.Content
		return ActionTerminalInput{Apply: func() {
			emu.Paste(content)
		}}

	case tea.MouseClickMsg:
		m, ok := t.translateMouse(uv.Mouse(msg))
		if !ok {
			return nil
		}
		return ActionTerminalInput{Apply: func() {
			emu.SendMouse(uv.MouseClickEvent(m))
		}}

	case tea.MouseReleaseMsg:
		m, ok := t.translateMouse(uv.Mouse(msg))
		if !ok {
			return nil
		}
		return ActionTerminalInput{Apply: func() {
			emu.SendMouse(uv.MouseReleaseEvent(m))
		}}

	case tea.MouseWheelMsg:
		m, ok := t.translateMouse(uv.Mouse(msg))
		if !ok {
			return nil
		}
		return ActionTerminalInput{Apply: func() {
			emu.SendMouse(uv.MouseWheelEvent(m))
		}}

	case tea.MouseMotionMsg:
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

// result builds the outcome for the finished session.
func (t *TerminalDialog) result() terminal.Result {
	return terminal.Result{
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

// renderHeader builds the one-line header: "$ command" on the left, the
// close hint on the right.
func (t *TerminalDialog) renderHeader(width int) string {
	sty := t.com.Styles

	hint := sty.Terminal.Hint.Render(t.closeKey.Help().Key + " " + t.closeKey.Help().Desc)
	hintWidth := lipgloss.Width(hint)

	commandWidth := max(width-hintWidth-2, 1)
	command := ansi.Truncate(t.command, commandWidth, "…")
	left := sty.Terminal.Header.Render("$ " + command)

	pad := max(width-lipgloss.Width(left)-hintWidth, 0)
	return left + strings.Repeat(" ", pad) + hint
}

// cursor returns the emulator cursor position offset by the content area.
func (t *TerminalDialog) cursor() *tea.Cursor {
	pos := t.session.Emulator().CursorPosition()
	return &tea.Cursor{
		Position: tea.Position{X: t.contentRect.Min.X + pos.X, Y: t.contentRect.Min.Y + pos.Y},
		Color:    t.com.Styles.Terminal.Cursor,
		Blink:    true,
	}
}
