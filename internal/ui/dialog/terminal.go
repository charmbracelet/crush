package dialog

import (
	"fmt"
	"image/color"
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
	// AgentDriven marks sessions the agent owns and drives itself: the
	// header names the agent as owner, and user keystrokes and mouse events
	// are refused so the panel is a read-only view of the agent's work.
	AgentDriven bool
}

// TerminalDialog embeds a live PTY session and forwards all input to it.
// The model owns the session lifecycle; the dialog only forwards input and
// paints the emulator.
type TerminalDialog struct {
	com         *common.Common
	session     *shell.InteractiveSession
	command     string
	agentDriven bool
	terminated  bool
	focused     bool
	contentRect uv.Rectangle
	closeKey    key.Binding
	fullKey     key.Binding
}

var _ Dialog = (*TerminalDialog)(nil)

// NewTerminalDialog creates an interactive terminal dialog around an
// already-running session.
func NewTerminalDialog(com *common.Common, session *shell.InteractiveSession, opts TerminalDialogOptions) *TerminalDialog {
	t := &TerminalDialog{
		com:         com,
		session:     session,
		command:     opts.Command,
		agentDriven: opts.AgentDriven,
	}
	t.closeKey = key.NewBinding(
		key.WithKeys("ctrl+q"),
		key.WithHelp("ctrl+q", "quit"),
	)
	// A user-owned terminal only toggles fullscreen; an agent-driven one
	// does too, it is just read-only for typing.
	t.fullKey = key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", "fullscreen"),
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
	// The session keeps the palette too, so OSC 104 can restore it when a
	// program resets colors it changed.
	t.session.SetPalette(sty.ANSI)
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
			// Routed through the session's input queue, so a wedged child
			// fails the write instead of hanging the UI goroutine.
			_ = t.session.SendKey(uv.KeyPressEvent(uv.Key(msg)))
		}}

	case tea.PasteMsg:
		if t.agentDriven {
			return nil
		}
		content := msg.Content
		return ActionTerminalInput{Apply: func() {
			// Paste is mode-aware: bracketed paste markers are added when
			// the child enabled them.
			_ = t.session.Paste(content)
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
			_ = t.session.SendMouse(uv.MouseClickEvent(m))
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
			_ = t.session.SendMouse(uv.MouseReleaseEvent(m))
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
			_ = t.session.SendMouse(uv.MouseWheelEvent(m))
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
			_ = t.session.SendMouse(uv.MouseMotionEvent(m))
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
	// Map onto the logical screen, not the panel slice, so events land
	// where the child expects them.
	m.X = m.X - t.contentRect.Min.X
	m.Y = m.Y - t.contentRect.Min.Y
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

	// A user-owned terminal must match the panel exactly so what the user
	// types lands where they see it. An agent-driven session keeps its own
	// logical size (the ghost fullscreen the host UI maintains) and the
	// panel shows a slice of it.
	if !t.agentDriven {
		if cols, rows := t.session.Size(); cols != contentWidth || rows != contentHeight {
			t.session.Resize(contentWidth, contentHeight)
		}
	}

	header := t.renderHeader(contentWidth)

	box := sty.Terminal.Border.
		Width(area.Dx()).
		Height(area.Dy()).
		Render(header)
	uv.NewStyledString(box).Draw(scr, area)

	t.contentRect = uv.Rect(area.Min.X+1, area.Min.Y+1+terminalHeaderLines, contentWidth, contentHeight)
	t.paintEmulator(scr)

	// The focused terminal shows the real host cursor; an unfocused one
	// paints a ghost cursor instead so the caret (often the agent's, for
	// read-only panels) stays visible.
	if !t.focused {
		t.paintGhostCursor(scr)
		return nil
	}
	return t.cursor()
}

// paintEmulator paints the panel from the session's emulator. The session
// can be logically larger than the panel: agent-driven terminals run at the
// full-window "ghost fullscreen" size so the agent reads the whole screen,
// while the panel shows the top slice of it. Cells carrying ANSI color
// codes are resolved through the emulator's palette; vt leaves those as
// indices, and the host terminal would paint them with its own palette
// instead of the theme's.
func (t *TerminalDialog) paintEmulator(scr uv.Screen) {
	emu := t.session.Emulator()
	area := t.contentRect
	fg := emu.ForegroundColor()
	bg := emu.BackgroundColor()

	// Top-left aligned: programs draw from the top-left corner, so the
	// first rows and columns of the logical screen fill the panel. Rows
	// beyond the logical screen height paint as blanks.
	for y := range area.Dy() {
		for x := range area.Dx() {
			cell := t.panelCell(emu, fg, bg, x, y)
			scr.SetCell(area.Min.X+x, area.Min.Y+y, cell)
		}
	}
}

// panelCell copies one emulator cell for the panel, substituting the
// emulator's default colors and resolving its ANSI color indices through
// the palette.
func (t *TerminalDialog) panelCell(emu *vt.SafeEmulator, fg, bg color.Color, x, y int) *uv.Cell {
	var cell *uv.Cell
	if src := emu.CellAt(x, y); src != nil {
		cell = src.Clone()
		if cell.Style.Fg == nil {
			cell.Style.Fg = fg
		}
		if cell.Style.Bg == nil {
			cell.Style.Bg = bg
		}
	} else {
		cell = uv.EmptyCell.Clone()
		cell.Style.Fg = fg
		cell.Style.Bg = bg
	}

	if resolved, ok := resolveIndexed(cell.Style.Fg, emu); ok {
		cell.Style.Fg = resolved
	}
	if resolved, ok := resolveIndexed(cell.Style.Bg, emu); ok {
		cell.Style.Bg = resolved
	}
	return cell
}

// viewOffset maps an emulator cell coordinate onto the panel, clipped to
// the visible slice. ok is false when the position lies outside it.
func (t *TerminalDialog) viewOffset(x, y int) (int, int, bool) {
	if x < 0 || y < 0 || x >= t.contentRect.Dx() || y >= t.contentRect.Dy() {
		return 0, 0, false
	}
	return t.contentRect.Min.X + x, t.contentRect.Min.Y + y, true
}

// focused reports whether the terminal currently owns the keyboard. The
// host UI sets it before each draw.
func (t *TerminalDialog) SetFocused(focused bool) {
	// Tell the child when it gains or loses focus, so programs that listen
	// for focus events (tmux, editors) see the pane come and go.
	if focused {
		t.session.Focus()
	} else {
		t.session.Blur()
	}
	t.focused = focused
}

// paintGhostCursor inverts the cell under the emulator cursor so the caret
// stays visible when the terminal does not own the keyboard and cannot
// show the real cursor. On a read-only panel this is what makes the
// agent's typing visible: the caret jumps to wherever it is writing.
func (t *TerminalDialog) paintGhostCursor(scr uv.Screen) {
	emu := t.session.Emulator()
	if emu.CursorHidden() {
		return
	}

	pos := emu.CursorPosition()
	sx, sy, ok := t.viewOffset(pos.X, pos.Y)
	if !ok {
		return
	}

	cell := scr.CellAt(sx, sy)
	if cell == nil {
		return
	}

	clone := cell.Clone()
	clone.Style.Fg, clone.Style.Bg = clone.Style.Bg, clone.Style.Fg
	scr.SetCell(sx, sy, clone)
}

// resolveIndexed maps a cell color that names an ANSI color slot onto the
// emulator's palette. True colors and unset colors report false, and so does
// a slot whose palette entry is the plain indexed color every terminal
// starts with.
func resolveIndexed(c color.Color, emu *vt.SafeEmulator) (color.Color, bool) {
	var index int
	switch c := c.(type) {
	case ansi.BasicColor:
		index = int(c)
	case ansi.IndexedColor:
		index = int(c)
	default:
		return nil, false
	}

	resolved := emu.IndexedColor(index)
	if resolved == nil || resolved == c {
		return nil, false
	}
	return resolved, true
}

// renderHeader builds the one-line header: the child's title (or the
// command) on the left, then the live status chip on the right. Keybinds
// live in the status bar and the editor hint row instead of on the panel.
func (t *TerminalDialog) renderHeader(width int) string {
	sty := t.com.Styles

	status := t.renderStatus()

	// Programs set their own titles (nvim, tmux, shells); when one does,
	// that is a better label than the raw command.
	label := "$ " + t.command
	if title := t.session.Title(); title != "" {
		label = title
	}

	commandWidth := max(width-lipgloss.Width(status)-2, 1)
	left := sty.Terminal.Header.Render(ansi.Truncate(label, commandWidth, "…"))

	pad := max(width-lipgloss.Width(left)-lipgloss.Width(status), 0)
	// The status chip can outgrow a narrow panel; keep the header to a
	// single row so the frame height stays predictable.
	return ansi.Truncate(left+strings.Repeat(" ", pad)+status, width, "…")
}

// renderStatus builds the live status chip shown on the right: who owns the
// session and whether the command is still running. The emulator below
// shows the activity itself; the chip makes it clear at a glance who is
// driving and that work is happening.
func (t *TerminalDialog) renderStatus() string {
	sty := t.com.Styles

	owner := sty.Terminal.Owner.Render("user")
	if t.agentDriven {
		owner = sty.Terminal.Owner.Render("agent")
	}

	state := sty.Terminal.Running.Render("● running")
	if t.session.Exited() {
		state = sty.Terminal.Exited.Render(fmt.Sprintf("exited %d", t.session.ExitCode()))
	}

	// A full-screen program is drawing; say so, since the panel then shows
	// a frame rather than a transcript.
	if t.session.InAltScreen() {
		state = sty.Terminal.Hint.Render("TUI") + " " + state
	}
	return owner + " " + state + " "
}

// cursor returns the emulator cursor, mapped through the visible slice;
// it returns nil when the child hid its cursor or when it sits outside the
// slice: showing a phantom cursor somewhere else on screen would only
// confuse.
func (t *TerminalDialog) cursor() *tea.Cursor {
	emu := t.session.Emulator()
	if emu.CursorHidden() {
		return nil
	}

	style, blink := emu.CursorStyle()
	pos := emu.CursorPosition()
	sx, sy, ok := t.viewOffset(pos.X, pos.Y)
	if !ok {
		return nil
	}

	return &tea.Cursor{
		Position: tea.Position{X: sx, Y: sy},
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
