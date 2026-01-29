package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/ui/spinner"
)

// Model is the Bubble Tea model for the example program.
type Model struct {
	spinner  spinner.Spinner
	quitting bool
}

// Init initializes the model. It satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return m.spinner.Step()
}

// Update updates the model per on incoming messages. It satisfies tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View renders the model to a string. It satisfies tea.Model.
func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	return tea.NewView(m.spinner.View())
}

func main() {
	if _, err := tea.NewProgram(Model{
		spinner: spinner.NewSpinner(),
	}).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
