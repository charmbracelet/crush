// Package termdialog provides a reusable dialog component for embedding
// terminal applications in the TUI.
package termdialog

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	// headerHeight is the height of the dialog header (title + padding).
	headerHeight = 2
	// fullscreenWidthBreakpoint is the width below which the dialog goes
	// fullscreen.
	fullscreenWidthBreakpoint = 125
	// fullscreenHeightBreakpoint is the height below which the dialog goes
	// fullscreen. Lazygit degrades significantly below 40 rows.
	fullscreenHeightBreakpoint = 40
)

// Config holds configuration for a terminal dialog.
type Config struct {
	// ID is the unique identifier for this dialog.
	ID dialogs.DialogID
	// Title is displayed in the dialog header.
	Title string
	// LoadingMsg is shown while the terminal is starting.
	LoadingMsg string
	// Term is the terminal to embed.
	Term *terminal.Terminal
	// OnClose is called when the dialog is closed (optional).
	OnClose func()
	// QuitHint is shown in the header (e.g., "q to close"). If empty, no hint is shown.
	QuitHint string
}

// Dialog is a dialog that embeds a terminal application.
type Dialog struct {
	id         dialogs.DialogID
	title      string
	loadingMsg string
	term       *terminal.Terminal
	onClose    func()
	quitHint   string

	wWidth     int
	wHeight    int
	width      int
	height     int
	fullscreen bool
}

// New creates a new terminal dialog with the given configuration.
func New(cfg Config) *Dialog {
	loadingMsg := cfg.LoadingMsg
	if loadingMsg == "" {
		loadingMsg = "Starting..."
	}

	return &Dialog{
		id:         cfg.ID,
		title:      cfg.Title,
		loadingMsg: loadingMsg,
		term:       cfg.Term,
		onClose:    cfg.OnClose,
		quitHint:   cfg.QuitHint,
	}
}

func (d *Dialog) Init() tea.Cmd {
	return nil
}

func (d *Dialog) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return d.handleResize(msg)

	case terminal.ExitMsg:
		return d, util.CmdHandler(dialogs.CloseDialogMsg{})

	case terminal.OutputMsg:
		if d.term.Closed() {
			return d, nil
		}
		return d, d.term.RefreshCmd()

	case tea.KeyPressMsg:
		return d.handleKey(msg)

	case tea.PasteMsg:
		d.term.SendPaste(msg.Content)
		return d, nil

	case tea.MouseMsg:
		return d.handleMouse(msg)
	}

	return d, nil
}

func (d *Dialog) handleResize(msg tea.WindowSizeMsg) (util.Model, tea.Cmd) {
	d.wWidth = msg.Width
	d.wHeight = msg.Height

	// Go fullscreen when window is below size breakpoints.
	d.fullscreen = msg.Width < fullscreenWidthBreakpoint || msg.Height < fullscreenHeightBreakpoint

	var outerWidth, outerHeight int
	if d.fullscreen {
		outerWidth = msg.Width
		outerHeight = msg.Height
	} else {
		// Dialog takes up 85% of the screen to show it's embedded.
		outerWidth = int(float64(msg.Width) * 0.85)
		outerHeight = int(float64(msg.Height) * 0.85)

		// Cap at reasonable maximums.
		if outerWidth > msg.Width-6 {
			outerWidth = msg.Width - 6
		}
		if outerHeight > msg.Height-4 {
			outerHeight = msg.Height - 4
		}
	}

	// Inner dimensions = outer - border (1 char each side = 2 total).
	d.width = max(outerWidth-2, 40)
	d.height = max(outerHeight-2, 10)

	// Terminal height excludes the header.
	termHeight := max(d.height-headerHeight, 5)

	// Start the terminal if not started.
	if !d.term.Started() && d.width > 0 && termHeight > 0 {
		if err := d.term.Resize(d.width, termHeight); err != nil {
			return d, util.ReportError(err)
		}
		if err := d.term.Start(); err != nil {
			return d, util.ReportError(err)
		}
		return d, tea.Batch(d.term.WaitCmd(), d.term.RefreshCmd())
	}

	// Resize existing terminal.
	if err := d.term.Resize(d.width, termHeight); err != nil {
		return d, util.ReportError(err)
	}
	return d, nil
}

func (d *Dialog) handleKey(msg tea.KeyPressMsg) (util.Model, tea.Cmd) {
	if msg.Text != "" {
		d.term.SendText(msg.Text)
	} else {
		d.term.SendKey(msg)
	}
	return d, nil
}

func (d *Dialog) handleMouse(msg tea.MouseMsg) (util.Model, tea.Cmd) {
	row, col := d.Position()

	// Adjust coordinates for dialog position.
	adjust := func(x, y int) (int, int) {
		return x - col - 1, y - row - 1 - headerHeight
	}

	switch ev := msg.(type) {
	case tea.MouseClickMsg:
		ev.X, ev.Y = adjust(ev.X, ev.Y)
		d.term.SendMouse(ev)
	case tea.MouseReleaseMsg:
		ev.X, ev.Y = adjust(ev.X, ev.Y)
		d.term.SendMouse(ev)
	case tea.MouseWheelMsg:
		ev.X, ev.Y = adjust(ev.X, ev.Y)
		d.term.SendMouse(ev)
	case tea.MouseMotionMsg:
		ev.X, ev.Y = adjust(ev.X, ev.Y)
		d.term.SendMouse(ev)
	}
	return d, nil
}

func (d *Dialog) View() string {
	t := styles.CurrentTheme()

	var termContent string
	if d.term.Started() {
		termContent = d.term.Render()
	} else {
		termContent = d.loadingMsg
	}

	// Build header with title and optional quit hint on the right.
	var header string
	if d.quitHint != "" {
		hintStyle := t.S().Base.Foreground(t.Secondary)
		hint := hintStyle.Render(d.quitHint)
		hintWidth := lipgloss.Width(hint)
		titleWidth := d.width - 2 - hintWidth - 1 // -1 for space between title and hint
		title := core.Title(d.title, titleWidth)
		headerContent := title + " " + hint
		header = t.S().Base.Padding(0, 1, 1, 1).Render(headerContent)
	} else {
		header = t.S().Base.Padding(0, 1, 1, 1).Render(core.Title(d.title, d.width-2))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, termContent)

	dialogStyle := t.S().Base.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)

	return dialogStyle.Render(content)
}

func (d *Dialog) Position() (int, int) {
	if d.fullscreen {
		return 0, 0
	}

	dialogWidth := d.width + 2
	dialogHeight := d.height + 2

	row := max((d.wHeight-dialogHeight)/2, 0)
	col := max((d.wWidth-dialogWidth)/2, 0)

	return row, col
}

func (d *Dialog) ID() dialogs.DialogID {
	return d.id
}

// Cursor returns the cursor position adjusted for the dialog's screen position.
// Returns nil if the terminal cursor is hidden or not available.
func (d *Dialog) Cursor() *tea.Cursor {
	x, y := d.term.CursorPosition()
	if x < 0 || y < 0 {
		return nil
	}

	t := styles.CurrentTheme()
	row, col := d.Position()
	cursor := tea.NewCursor(x, y)
	cursor.X += col + 1
	cursor.Y += row + 1 + headerHeight
	cursor.Color = t.Secondary
	cursor.Shape = tea.CursorBlock
	cursor.Blink = true
	return cursor
}

func (d *Dialog) Close() tea.Cmd {
	_ = d.term.Close()

	if d.onClose != nil {
		d.onClose()
	}

	return nil
}
