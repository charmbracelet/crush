package anim

import (
	"cmp"
	"image/color"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

type noAnim struct {
	Color    color.Color
	rendered string
	id       int
}

func newStatic(label string, foreground color.Color) Spinner {
	a := &noAnim{Color: foreground}
	a.SetLabel(label)
	a.id = nextID()
	return a
}

func (s *noAnim) SetLabel(label string) {
	s.rendered = lipgloss.NewStyle().
		Foreground(s.Color).
		Render(cmp.Or(label, "Working") + ellipsisFrames[2])
}

func (s noAnim) Init() tea.Cmd { return stepCmd(s.id) }
func (s *noAnim) View() string { return s.rendered }
func (s *noAnim) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	switch msg := msg.(type) {
	case StepMsg:
		if msg.id != s.id {
			// Reject messages that are not for this instance.
			return s, nil
		}
		return s, stepCmd(s.id)
	default:
		return s, nil
	}
}
