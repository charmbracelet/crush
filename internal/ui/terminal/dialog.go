package terminal

import (
	"image/color"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/exp/charmtone"
)

const (
	// DialogID is the unique identifier for the terminal dialog.
	DialogID = "terminal"
	// headerHeight is the height of the dialog header (title + padding).
	headerHeight = 2
)

// DialogConfig holds configuration for the terminal dialog.
type DialogConfig struct {
	Title    string
	QuitHint string
	Term     *Terminal
	OnClose  func()
}

// Dialog wraps a Terminal in a dialog.Dialog compatible component.
type Dialog struct {
	id       string
	title    string
	quitHint string
	term     *Terminal
	onClose  func()

	dialogStyle        lipgloss.Style
	titleGradFromColor color.Color
	titleGradToColor   color.Color
	sty                *styles.Styles
	width              int
	height             int
	termArea           uv.Rectangle // last drawn terminal content area
}

// NewDialog creates a new terminal dialog.
func NewDialog(cfg DialogConfig) *Dialog {
	return &Dialog{
		id:       DialogID,
		title:    cfg.Title,
		quitHint: cfg.QuitHint,
		term:     cfg.Term,
		onClose:  cfg.OnClose,
	}
}

// SetStyles sets the styles and dialog view style. Call before Draw.
func (d *Dialog) SetStyles(sty *styles.Styles) {
	d.sty = sty
	d.dialogStyle = sty.Dialog.View
	d.titleGradFromColor = sty.Dialog.TitleGradFromColor
	d.titleGradToColor = sty.Dialog.TitleGradToColor
}

// ID implements dialog.Dialog.
func (d *Dialog) ID() string { return d.id }

// HandleMsg implements dialog.Dialog.
func (d *Dialog) HandleMsg(msg tea.Msg) dialog.Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		d.term.SendKey(msg)
	case tea.PasteMsg:
		d.term.SendPaste(msg.Content)
	case tea.MouseClickMsg:
		d.adjustAndSendMouse(msg)
	case tea.MouseReleaseMsg:
		d.adjustAndSendMouse(msg)
	case tea.MouseWheelMsg:
		d.adjustAndSendMouse(msg)
	case tea.MouseMotionMsg:
		d.adjustAndSendMouse(msg)
	}
	return nil
}

// adjustAndSendMouse adjusts mouse coordinates from screen space to
// terminal content space and forwards the event to the terminal.
func (d *Dialog) adjustAndSendMouse(msg tea.MouseMsg) {
	m := msg.Mouse()
	m.X -= d.termArea.Min.X
	m.Y -= d.termArea.Min.Y
	// Clamp to terminal bounds.
	if m.X < 0 || m.Y < 0 || m.X >= d.termArea.Dx() || m.Y >= d.termArea.Dy() {
		return
	}
	switch ev := msg.(type) {
	case tea.MouseClickMsg:
		ev.X, ev.Y = m.X, m.Y
		d.term.SendMouse(ev)
	case tea.MouseReleaseMsg:
		ev.X, ev.Y = m.X, m.Y
		d.term.SendMouse(ev)
	case tea.MouseWheelMsg:
		ev.X, ev.Y = m.X, m.Y
		d.term.SendMouse(ev)
	case tea.MouseMotionMsg:
		ev.X, ev.Y = m.X, m.Y
		d.term.SendMouse(ev)
	}
}

// DialogContentSize computes the terminal content dimensions (inside the
// border, below the header) for a given total screen size. This must
// stay in sync with Dialog.Draw's sizing logic so the shell's first
// output wraps at the same width it is later displayed at.
func DialogContentSize(screenW, screenH int) (w, h int) {
	dialogW := min(max(int(float64(screenW)*0.8), 20), screenW)
	dialogH := min(max(int(float64(screenH)*0.8), 10), screenH)
	const borderSize = 1
	w = max(dialogW-borderSize*2, 1)
	h = max(dialogH-borderSize*2-headerHeight, 1)
	return w, h
}

// Draw implements dialog.Dialog. It renders the terminal content
// inside a dialog-style frame that fills most of the screen.
func (d *Dialog) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	// Size the dialog to fill ~80% of the screen.
	dialogW := min(max(int(float64(area.Dx())*0.8), 20), area.Dx())
	dialogH := min(max(int(float64(area.Dy())*0.8), 10), area.Dy())

	// Center using the same helper as all other Crush dialogs.
	dialogArea := common.CenterRect(area, dialogW, dialogH)

	// Border is always 1 cell for RoundedBorder. Content sits inside.
	const borderSize = 1
	contentW := max(dialogArea.Dx()-borderSize*2, 1)
	contentH := max(dialogArea.Dy()-borderSize*2, 1)

	// Build header first so we know its actual height before sizing the terminal.
	var header string
	actualHeaderH := headerHeight
	if d.sty != nil {
		titlePadH := d.sty.Dialog.Title.GetHorizontalFrameSize()
		availTitleW := max(contentW-titlePadH, 1)

		hintStr := ""
		hintWidth := 0
		if d.quitHint != "" {
			hintStyle := d.sty.Dialog.Help.ShortKey
			hintStr = hintStyle.Render(d.quitHint)
			hintWidth = lipgloss.Width(hintStr) + 1
		}
		titleW := max(availTitleW-hintWidth, 1)
		title := common.DialogTitle(d.sty, d.title, titleW, d.titleGradFromColor, d.titleGradToColor)
		headerLine := title
		if hintStr != "" {
			headerLine += " " + hintStr
		}
		header = d.sty.Dialog.Title.PaddingBottom(1).Render(headerLine)
		actualHeaderH = lipgloss.Height(header)
	} else {
		header = d.title + "\n"
	}

	// Now compute terminal size using the actual header height.
	termH := max(contentH-actualHeaderH, 1)

	// Resize terminal if needed.
	if contentW != d.width || termH != d.height {
		d.width = contentW
		d.height = termH
		if d.term.Started() {
			_ = d.term.Resize(contentW, termH)
		}
	}

	// Define areas relative to dialogArea + border offset.
	// Note: uv.Rect takes (x, y, width, height), not (minX, minY, maxX, maxY).
	contentMinX := dialogArea.Min.X + borderSize
	contentMinY := dialogArea.Min.Y + borderSize

	headerArea := uv.Rect(contentMinX, contentMinY, contentW, actualHeaderH)
	d.termArea = uv.Rect(contentMinX, contentMinY+actualHeaderH, contentW, termH)

	// Draw border cell-by-cell rather than using lipgloss.Render because
	// the terminal content interior must be drawn via DrawTo (cell-by-cell
	// to prevent background bleed). A lipgloss-rendered border would
	// produce a complete string that can't be partially overwritten by the
	// emulator's cell output without z-order issues.
	border := lipgloss.RoundedBorder()
	fg := d.sty.Dialog.View.GetBorderTopForeground()
	if fg == nil {
		fg = charmtone.Charple
	}
	bCell := func(c string) *uv.Cell {
		return &uv.Cell{Content: c, Style: uv.Style{Fg: fg}, Width: 1}
	}
	bx0 := dialogArea.Min.X
	by0 := dialogArea.Min.Y
	bx1 := dialogArea.Max.X - 1
	by1 := dialogArea.Max.Y - 1
	scr.SetCell(bx0, by0, bCell(border.TopLeft))
	for x := bx0 + 1; x < bx1; x++ {
		scr.SetCell(x, by0, bCell(border.Top))
	}
	scr.SetCell(bx1, by0, bCell(border.TopRight))
	for y := by0 + 1; y < by1; y++ {
		scr.SetCell(bx0, y, bCell(border.Left))
		scr.SetCell(bx1, y, bCell(border.Right))
	}
	scr.SetCell(bx0, by1, bCell(border.BottomLeft))
	for x := bx0 + 1; x < bx1; x++ {
		scr.SetCell(x, by1, bCell(border.Bottom))
	}
	scr.SetCell(bx1, by1, bCell(border.BottomRight))

	// Draw header.
	uv.NewStyledString(header).Draw(scr, headerArea)

	// Draw terminal content directly from the emulator, clipped to
	// termArea. Drawing cell-by-cell guarantees no styled background can
	// land past the dialog border.
	if !d.term.Started() || d.term.Closed() || !d.term.DrawTo(scr, d.termArea) {
		uv.NewStyledString("Starting...").Draw(scr, d.termArea)
	}

	// Position cursor.
	cx, cy := d.term.CursorPosition()
	if cx < 0 || cy < 0 {
		return nil
	}
	return tea.NewCursor(d.termArea.Min.X+cx, d.termArea.Min.Y+cy)
}

// Term returns the underlying Terminal for lifecycle management.
func (d *Dialog) Term() *Terminal { return d.term }

// Close cleans up the terminal and calls OnClose if set.
func (d *Dialog) Close() {
	_ = d.term.Close()
	if d.onClose != nil {
		d.onClose()
	}
}
