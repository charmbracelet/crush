package logout

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type confirmKeyMap struct {
	Switch  key.Binding
	Confirm key.Binding
	Yes     key.Binding
	No      key.Binding
	Cancel  key.Binding
}

func defaultConfirmKeyMap() confirmKeyMap {
	return confirmKeyMap{
		Switch:  key.NewBinding(key.WithKeys("left", "right", "h", "l"), key.WithHelp("←/→", "switch")),
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Yes:     key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "yes")),
		No:      key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n", "no")),
		Cancel:  key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("esc", "cancel")),
	}
}

func (k confirmKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Switch, k.Confirm, k.Yes, k.No, k.Cancel}
}

func (k confirmKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Switch, k.Confirm, k.Yes, k.No, k.Cancel}}
}

// confirmModel is the Bubble Tea model for a yes/no question rendered as
// selectable buttons. No is selected by default for safety.
type confirmModel struct {
	question   string
	answered   bool
	yes        bool
	quitting   bool
	blankLines int

	help   help.Model
	keymap confirmKeyMap
	width  int
}

func newConfirmModel(question string) confirmModel {
	return confirmModel{
		question: question,
		help:     help.New(),
		keymap:   defaultConfirmKeyMap(),
	}
}

// runConfirm runs the yes/no TUI and reports the user's answer.
func runConfirm(question string) (bool, error) {
	p := tea.NewProgram(newConfirmModel(question))
	final, err := p.Run()
	if err != nil {
		return false, err
	}
	m, ok := final.(confirmModel)
	if !ok {
		return false, errors.New("unexpected confirm program model")
	}
	return m.answered && m.yes, nil
}

// Init initializes the confirm TUI.
func (m confirmModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the confirm TUI.
func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.help.SetWidth(msg.Width)
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keymap.Cancel):
			m.quitting = true
			m.blankLines = m.contentHeight()
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Yes):
			m.answered = true
			m.yes = true
			m.quitting = true
			m.blankLines = m.contentHeight()
			return m, tea.Quit
		case key.Matches(msg, m.keymap.No):
			m.answered = true
			m.yes = false
			m.quitting = true
			m.blankLines = m.contentHeight()
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Confirm):
			m.answered = true
			m.quitting = true
			m.blankLines = m.contentHeight()
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Switch):
			m.yes = !m.yes
			return m, nil
		}
	}
	return m, nil
}

// content renders the full confirm view. The trailing zero-width space
// keeps the final blank line alive (the renderer erases trailing empty
// lines).
func (m confirmModel) content() string {
	var b strings.Builder

	b.WriteString("\n  ")
	b.WriteString(questionStyle.Render(m.question))
	b.WriteString("\n\n  ")
	b.WriteString(renderButton("Yes", m.yes))
	b.WriteString(" ")
	b.WriteString(renderButton("No", !m.yes))
	b.WriteString("\n\n  ")
	b.WriteString(m.help.View(m.keymap))
	b.WriteString("\n")
	return b.String()
}

// contentHeight returns the height of the current view in lines.
func (m confirmModel) contentHeight() int {
	return lipgloss.Height(m.content())
}

// View renders the confirm TUI.
func (m confirmModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	return tea.NewView(m.content())
}
