package dialog

import (
	"fmt"
	"image"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// QuitID is the identifier for the quit dialog.
const QuitID = "quit"

// Dialog content lines.
const (
	quitQuestion    = "Are you sure you want to quit?"
	quitHintLineOne = "To quit without confirmation"
	quitHintLineTwo = "press ctrl+c twice."
)

// Quit represents a confirmation dialog for quitting the application.
type Quit struct {
	com        *common.Common
	selectedNo bool // true if "No" button is selected
	compositor *lipgloss.Compositor
	hoverX     int
	hoverY     int
	keyMap     struct {
		LeftRight,
		EnterSpace,
		Yes,
		No,
		Tab,
		Close,
		Quit key.Binding
	}
}

var _ Dialog = (*Quit)(nil)

// NewQuit creates a new quit confirmation dialog.
func NewQuit(com *common.Common) *Quit {
	q := &Quit{
		com:        com,
		selectedNo: true,
	}
	q.keyMap.LeftRight = key.NewBinding(
		key.WithKeys("left", "right"),
		key.WithHelp("←/→", "switch options"),
	)
	q.keyMap.EnterSpace = key.NewBinding(
		key.WithKeys("enter", " ", "space"),
		key.WithHelp("enter/space", "confirm"),
	)
	q.keyMap.Yes = key.NewBinding(
		key.WithKeys("y", "Y", "ctrl+c"),
		key.WithHelp("y/Y/ctrl+c", "yes"),
	)
	q.keyMap.No = key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n/N", "no"),
	)
	q.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch options"),
	)
	q.keyMap.Close = CloseKey
	q.keyMap.Quit = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	)
	return q
}

// ID implements [Model].
func (*Quit) ID() string {
	return QuitID
}

// HandleMsg implements [Model].
func (q *Quit) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.MouseMotionMsg:
		q.hoverX, q.hoverY = msg.X, msg.Y
	case tea.MouseClickMsg:
		return q.handleMouseClick(msg)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, q.keyMap.Quit):
			return ActionQuit{}
		case key.Matches(msg, q.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, q.keyMap.LeftRight, q.keyMap.Tab):
			q.selectedNo = !q.selectedNo
		case key.Matches(msg, q.keyMap.EnterSpace):
			if !q.selectedNo {
				return ActionQuit{}
			}
			return ActionClose{}
		case key.Matches(msg, q.keyMap.Yes):
			return ActionQuit{}
		case key.Matches(msg, q.keyMap.No, q.keyMap.Close):
			return ActionClose{}
		}
	}

	return nil
}

// handleMouseClick selects the clicked button. Clicking the already
// selected button activates it.
func (q *Quit) handleMouseClick(msg tea.MouseClickMsg) Action {
	if msg.Button != tea.MouseLeft {
		return nil
	}
	switch common.HitButtonIndex(q.compositor, msg.X, msg.Y) {
	case 0: // "Yep!"
		if q.selectedNo {
			q.selectedNo = false
			return nil
		}
		return ActionQuit{}
	case 1: // "Nope"
		if !q.selectedNo {
			q.selectedNo = true
			return nil
		}
		return ActionClose{}
	}
	return nil
}

// Draw implements [Dialog].
func (q *Quit) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	const (
		// buttonLine is the index of the content line reserved for the
		// buttons, which are drawn as layers on top of the frame.
		buttonLine = 2
	)
	var (
		baseStyle = q.com.Styles.Dialog.Quit.Content
		hintStyle = q.com.Styles.Dialog.Quit.Hint
	)
	buttonOpts := []common.ButtonOpts{
		{Text: "Yep!", Selected: !q.selectedNo, Padding: 3},
		{Text: "Nope", Selected: q.selectedNo, Padding: 3},
	}
	if hovered := common.HitButtonIndex(q.compositor, q.hoverX, q.hoverY); hovered >= 0 {
		buttonOpts[hovered].Hovered = true
	}
	buttonViews := make([]string, len(buttonOpts))
	for i, o := range buttonOpts {
		buttonViews[i] = common.Button(q.com.Styles, o)
	}
	buttons := strings.Join(buttonViews, " ")
	buttonsWidth := lipgloss.Width(buttons)

	// renderContent builds the dialog body around the given button row. A
	// blank row of the same width reserves space for the buttons, which are
	// painted as layers so their bounds double as mouse hit regions.
	renderContent := func(buttonRow string) string {
		return baseStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Center,
				quitQuestion,
				"",
				buttonRow,
				"",
				hintStyle.Render(quitHintLineOne),
				hintStyle.Render(quitHintLineTwo),
			),
		)
	}

	content := renderContent(strings.Repeat(" ", buttonsWidth))

	frameStyle := q.com.Styles.Dialog.Quit.Frame
	maxWidth := area.Dx() - frameStyle.GetHorizontalBorderSize()
	if maxWidth < lipgloss.Width(content) {
		frameStyle = frameStyle.Padding(1, 0)
	}
	view := frameStyle.Render(content)

	width, height := lipgloss.Size(view)
	width = min(width, area.Dx())
	height = min(height, area.Dy())
	center := common.CenterRect(area, width, height)

	// Offset the content by every frame the two styles add around it.
	contentX := center.Min.X +
		baseStyle.GetMarginLeft() + baseStyle.GetBorderLeftSize() + baseStyle.GetPaddingLeft() +
		frameStyle.GetMarginLeft() + frameStyle.GetBorderLeftSize() + frameStyle.GetPaddingLeft()
	contentY := center.Min.Y +
		baseStyle.GetMarginTop() + baseStyle.GetBorderTopSize() + baseStyle.GetPaddingTop() +
		frameStyle.GetMarginTop() + frameStyle.GetBorderTopSize() + frameStyle.GetPaddingTop()
	buttonsX := contentX + (lipgloss.Width(content)-buttonsWidth)/2
	buttonsY := contentY + buttonLine

	q.compositor = nil
	buttonsRect := image.Rect(buttonsX, buttonsY, buttonsX+buttonsWidth, buttonsY+1)
	if buttonsRect.In(center) {
		DrawCenter(scr, area, view)
		q.compositor = drawButtons(scr, buttonsX, buttonsY, buttonViews)
	} else {
		// The buttons don't fit on screen as layers, so render them inline
		// as part of the content instead: the user always sees them.
		DrawCenter(scr, area, frameStyle.Render(renderContent(buttons)))
	}
	return nil
}

// drawButtons paints the buttons as layers at the given position and returns
// a compositor whose layer bounds serve as mouse hit regions.
func drawButtons(scr uv.Screen, x, y int, views []string) *lipgloss.Compositor {
	layers := make([]*lipgloss.Layer, len(views))
	bx := x
	for i, v := range views {
		layers[i] = lipgloss.NewLayer(v).X(bx).Y(y).ID(fmt.Sprintf("btn_%d", i))
		bx += lipgloss.Width(v) + 1 // one cell between buttons
	}
	compositor := lipgloss.NewCompositor(layers...)
	compositor.Draw(scr, scr.Bounds())
	return compositor
}

// ShortHelp implements [help.KeyMap].
func (q *Quit) ShortHelp() []key.Binding {
	return []key.Binding{
		q.keyMap.LeftRight,
		q.keyMap.EnterSpace,
	}
}

// FullHelp implements [help.KeyMap].
func (q *Quit) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{q.keyMap.LeftRight, q.keyMap.EnterSpace, q.keyMap.Yes, q.keyMap.No},
		{q.keyMap.Tab, q.keyMap.Close},
	}
}
