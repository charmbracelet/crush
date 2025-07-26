package main

import (
	"fmt"
	"image/color"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
	anim "github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

type model struct {
	anim     tea.Model
	bgColor  color.Color
	quitting bool
	w, h     int
}

func (m model) Init() tea.Cmd {
	return m.anim.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
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
	if m.w == 0 || m.h == 0 {
		return tea.NewView("")
	}

	v := tea.NewView("")
	v.BackgroundColor = m.bgColor

	if m.quitting {
		return v
	}

	if a, ok := m.anim.(*anim.Anim); ok {
		l := lipgloss.NewLayer(a.View()).
			Width(a.Width()).
			X(m.w/2 - a.Width()/2).
			Y(m.h / 2)

		v = tea.NewView(lipgloss.NewCanvas(l))
		v.BackgroundColor = m.bgColor
		return v
	}
	return v
}

func main() {
	t := styles.CurrentTheme()
	p := tea.NewProgram(model{
		bgColor: t.BgBase,
		anim: anim.New(anim.Settings{
			Label:       "Hello",
			Size:        50,
			LabelColor:  t.FgBase,
			GradColorA:  t.Primary,
			GradColorB:  t.Secondary,
			CycleColors: true,
		}),
	}, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Uh oh: %v\n", err)
		os.Exit(1)
	}
}
