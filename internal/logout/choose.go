package logout

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type chooseKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Confirm key.Binding
	Cancel  key.Binding
}

func defaultChooseKeyMap() chooseKeyMap {
	return chooseKeyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/↓", "select")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("", "")),
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Cancel:  key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("esc", "cancel")),
	}
}

func (k chooseKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Confirm, k.Cancel}
}

func (k chooseKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Confirm, k.Cancel}}
}

// chooseModel is the Bubble Tea model for a multiple choice question with
// a ">" cursor.
type chooseModel struct {
	question string
	options  []string
	cursor   int
	chosen   int

	help   help.Model
	keymap chooseKeyMap

	quitting   bool
	blankLines int
	width      int
}

func newChooseModel(question string, options []string) chooseModel {
	return chooseModel{
		question: question,
		options:  options,
		chosen:   -1,
		help:     help.New(),
		keymap:   defaultChooseKeyMap(),
	}
}

// runChoose runs the multiple choice TUI and returns the selected option
// index, or -1 when canceled.
func runChoose(question string, options []string) (int, error) {
	p := tea.NewProgram(newChooseModel(question, options))
	final, err := p.Run()
	if err != nil {
		return -1, err
	}
	m, ok := final.(chooseModel)
	if !ok {
		return -1, errors.New("unexpected choose program model")
	}
	return m.chosen, nil
}

// Init initializes the choose TUI.
func (m chooseModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the choose TUI.
func (m chooseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case key.Matches(msg, m.keymap.Confirm):
			m.chosen = m.cursor
			m.quitting = true
			m.blankLines = m.contentHeight()
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Up):
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.options) - 1
			}
			return m, nil
		case key.Matches(msg, m.keymap.Down):
			m.cursor++
			if m.cursor >= len(m.options) {
				m.cursor = 0
			}
			return m, nil
		}
	}
	return m, nil
}

// content renders the full choose view. The trailing zero-width space
// keeps the final blank line alive (the renderer erases trailing empty
// lines).
func (m chooseModel) content() string {
	var b strings.Builder

	b.WriteString("\n  ")
	b.WriteString(questionStyle.Render(m.question))
	b.WriteString("\n\n")
	for i, option := range m.options {
		if i == m.cursor {
			b.WriteString("  " + choiceSelectedStyle.Render("> "+option))
		} else {
			b.WriteString("    " + option)
		}
		if i < len(m.options)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n\n  ")
	b.WriteString(m.help.View(m.keymap))
	b.WriteString("\n")
	return b.String()
}

// contentHeight returns the height of the current view in lines.
func (m chooseModel) contentHeight() int {
	return lipgloss.Height(m.content())
}

// View renders the choose TUI.
func (m chooseModel) View() tea.View {
	if m.quitting {
		// Render the same number of blank lines as the last frame so the
		// renderer overwrites it and the TUI disappears on exit. A truly
		// empty view doesn't work under Fang's wrapped output: the renderer
		// drops the touched-lines buffer when the height shrinks to zero
		// and leaves the frame on screen.
		return tea.NewView("")
	}
	return tea.NewView(m.content())
}
