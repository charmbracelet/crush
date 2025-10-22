package dialog

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Quit represents a confirmation dialog for quitting the application.
type Quit struct {
	keyMap     QuitKeyMap
	selectedNo bool // true if "No" button is selected
}

// NewQuit creates a new quit confirmation dialog.
func NewQuit() *Quit {
	q := &Quit{
		keyMap: DefaultQuitKeyMap(),
	}
	return q
}

// ID implements [Model].
func (*Quit) ID() string {
	return "quit"
}

// Update implements [Model].
func (q *Quit) Update(msg tea.Msg) (Model, tea.Cmd) {
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

	baseStyle := lipgloss.NewStyle()
	yesStyle := lipgloss.NewStyle()
	noStyle := yesStyle

	if q.selectedNo {
		noStyle = noStyle.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("15"))
		yesStyle = yesStyle.Background(lipgloss.Color("15"))
	} else {
		yesStyle = yesStyle.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("15"))
		noStyle = noStyle.Background(lipgloss.Color("15"))
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

	quitDialogStyle := baseStyle.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("15"))

	return quitDialogStyle.Render(content)
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
