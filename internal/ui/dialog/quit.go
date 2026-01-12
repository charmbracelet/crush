package dialog

import (
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
	}

	return nil
}

// Draw implements [Dialog].
func (q *Quit) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	const question = "Are you sure you want to quit?"
	baseStyle := q.com.Styles.Base
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

	view := q.com.Styles.BorderFocus.Render(content)
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
