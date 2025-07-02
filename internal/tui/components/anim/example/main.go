package main

import (
	"fmt"
	"image/color"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
	anim "github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

type model struct {
	anim     tea.Model
	bgColor  color.Color
	quitting bool
}

func (m model) Init() tea.Cmd {
	return m.anim.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		default:
			return m, nil
		}
	case anim.StepMsg:
		var cmd tea.Cmd
		m.anim, cmd = m.anim.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m model) View() tea.View {
	// XXX tea.View() needs a content setter.
	v := tea.NewView("")
	v.SetBackgroundColor(m.bgColor)
	if m.quitting {
		return v
	}
	if a, ok := m.anim.(anim.Anim); ok {
		v = tea.NewView(a.View().String() + "\n")
		v.SetBackgroundColor(m.bgColor)
		return v
	}
	return v
}

func main() {
	t := styles.CurrentTheme()
	p := tea.NewProgram(model{
		bgColor: t.BgBase,
		anim:    anim.New(50, "Hello", t),
	})

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Uh oh: %v\n", err)
		os.Exit(1)
	}
}
