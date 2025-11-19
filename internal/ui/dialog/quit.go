package dialog

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// QuitDialogID is the identifier for the quit dialog.
const QuitDialogID = "quit"

// QuitDialogKeyMap represents key bindings for the quit dialog.
type QuitDialogKeyMap struct {
	LeftRight,
	EnterSpace,
	Yes,
	No,
	Tab,
	Close key.Binding
}

// DefaultQuitKeyMap returns the default key bindings for the quit dialog.
func DefaultQuitKeyMap() QuitDialogKeyMap {
	return QuitDialogKeyMap{
		LeftRight: key.NewBinding(
			key.WithKeys("left", "right"),
			key.WithHelp("←/→", "switch options"),
		),
		EnterSpace: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "confirm"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y", "Y", "ctrl+c"),
			key.WithHelp("y/Y/ctrl+c", "yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n/N", "no"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch options"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// Quit represents a confirmation dialog for quitting the application.
type Quit struct {
	com        *common.Common
	keyMap     QuitDialogKeyMap
	selectedNo bool // true if "No" button is selected
}

// NewQuit creates a new quit confirmation dialog.
func NewQuit(com *common.Common) *Quit {
	q := &Quit{
		com:        com,
		keyMap:     DefaultQuitKeyMap(),
		selectedNo: true,
	}
	return q
}

// ID implements [Model].
func (*Quit) ID() string {
	return QuitDialogID
}

// Update implements [Model].
func (q *Quit) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, q.keyMap.LeftRight, q.keyMap.Tab):
			q.selectedNo = !q.selectedNo
			return q, nil
		case key.Matches(msg, q.keyMap.EnterSpace):
			if !q.selectedNo {
				return q, tea.Quit
			}
			return nil, nil
		case key.Matches(msg, q.keyMap.Yes):
			return q, tea.Quit
		case key.Matches(msg, q.keyMap.No, q.keyMap.Close):
			return nil, nil
		}
	}

	return q, nil
}

// View implements [Model].
func (q *Quit) View() string {
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

	return q.com.Styles.BorderFocus.Render(content)
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
