package dialog

import (
	"image"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// QuitID is the identifier for the quit dialog.
const QuitID = "quit"

// Quit represents a confirmation dialog for quitting the application.
type Quit struct {
	com        *common.Common
	selectedNo bool // true if "No" button is selected
	keyMap     struct {
		LeftRight,
		EnterSpace,
		Yes,
		No,
		Tab,
		Close,
		Quit key.Binding
	}

	// Mouse click hitboxes for the two response buttons ("Yep!",
	// "Nope"). Computed during Draw().
	buttonsHitboxesValid bool
	buttonsHitRects      [2]image.Rectangle
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
		key.WithKeys("enter", " "),
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
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft || !q.buttonsHitboxesValid {
			break
		}
		pt := image.Pt(msg.X, msg.Y)
		switch {
		case pt.In(q.buttonsHitRects[0]):
			return ActionQuit{}
		case pt.In(q.buttonsHitRects[1]):
			return ActionClose{}
		}
	}

	return nil
}

// Draw implements [Dialog].
func (q *Quit) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	const question = "Are you sure you want to quit?"
	baseStyle := q.com.Styles.Dialog.Quit.Content
	buttonOpts := []common.ButtonOpts{
		{Text: "Yep!", Selected: !q.selectedNo, Padding: 3},
		{Text: "Nope", Selected: q.selectedNo, Padding: 3},
	}
	buttons := common.ButtonGroup(q.com.Styles, buttonOpts, " ")
	content := baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			question,
			"",
			buttons,
		),
	)

	view := q.com.Styles.Dialog.Quit.Frame.Render(content)

	// Compute click hitboxes for the response buttons.
	q.buttonsHitboxesValid = false
	q.buttonsHitRects = [2]image.Rectangle{}

	viewW, _ := lipgloss.Size(view)
	dialogRect := common.CenterRect(area, viewW, lipgloss.Height(view))

	frameStyle := q.com.Styles.Dialog.Quit.Frame
	innerMinX := dialogRect.Min.X + frameStyle.GetHorizontalFrameSize()/2
	innerMinY := dialogRect.Min.Y + frameStyle.GetVerticalFrameSize()/2

	// Buttons are at line 2 within inner content (question=0,
	// blank=1, buttons=2).
	buttonTopInInner := 2
	yButtonsTop := innerMinY + buttonTopInInner

	b0 := common.Button(q.com.Styles, buttonOpts[0])
	b1 := common.Button(q.com.Styles, buttonOpts[1])
	w0 := lipgloss.Width(b0)
	w1 := lipgloss.Width(b1)
	spacingW := lipgloss.Width(" ")
	buttonGroupW := w0 + spacingW + w1

	contentW := lipgloss.Width(content)
	buttonStartX := innerMinX + (contentW-buttonGroupW)/2

	const hitboxPad = 1
	x := buttonStartX
	q.buttonsHitRects[0] = image.Rect(
		max(0, x-hitboxPad),
		max(0, yButtonsTop-hitboxPad),
		x+w0+hitboxPad,
		yButtonsTop+lipgloss.Height(b0)+hitboxPad,
	)
	x += w0 + spacingW
	q.buttonsHitRects[1] = image.Rect(
		max(0, x-hitboxPad),
		max(0, yButtonsTop-hitboxPad),
		x+w1+hitboxPad,
		yButtonsTop+lipgloss.Height(b1)+hitboxPad,
	)
	q.buttonsHitboxesValid = true

	DrawCenter(scr, area, view)
	return nil
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
