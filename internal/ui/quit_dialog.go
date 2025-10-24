package ui

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

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

// QuitDialog represents a confirmation dialog for quitting the application.
type QuitDialog struct {
	com        *Common
	keyMap     QuitDialogKeyMap
	selectedNo bool // true if "No" button is selected
}

// NewQuitDialog creates a new quit confirmation dialog.
func NewQuitDialog(com *Common) *QuitDialog {
	q := &QuitDialog{
		com:    com,
		keyMap: DefaultQuitKeyMap(),
	}
	return q
}

// ID implements [Model].
func (*QuitDialog) ID() string {
	return "quit"
}

// Update implements [Model].
func (q *QuitDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
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
func (q *QuitDialog) View() string {
	const question = "Are you sure you want to quit?"
	baseStyle := q.com.Styles.Base
	yesStyle := q.com.Styles.ButtonSelected
	noStyle := q.com.Styles.ButtonUnselected

	if q.selectedNo {
		noStyle = q.com.Styles.ButtonSelected
		yesStyle = q.com.Styles.ButtonUnselected
	}

	const horizontalPadding = 3
	yesButton := yesStyle.PaddingLeft(horizontalPadding).Underline(true).Render("Y") +
		yesStyle.PaddingRight(horizontalPadding).Render("ep!")
	noButton := noStyle.PaddingLeft(horizontalPadding).Underline(true).Render("N") +
		noStyle.PaddingRight(horizontalPadding).Render("ope")

	buttons := baseStyle.Width(lipgloss.Width(question)).Align(lipgloss.Right).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, yesButton, "  ", noButton),
	)

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
func (q *QuitDialog) ShortHelp() []key.Binding {
	return []key.Binding{
		q.keyMap.LeftRight,
		q.keyMap.EnterSpace,
	}
}

// FullHelp implements [help.KeyMap].
func (q *QuitDialog) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{q.keyMap.LeftRight, q.keyMap.EnterSpace, q.keyMap.Yes, q.keyMap.No},
		{q.keyMap.Tab, q.keyMap.Close},
	}
}
