package format

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

// Spinner wraps the bubbles spinner for non-interactive mode
type Spinner struct {
	done chan struct{}
	prog *tea.Program
}

// spinnerModel is the tea.Model for the spinner
type spinnerModel struct {
	animation anim.Anim
	quitting  bool
}

func (m spinnerModel) Init() tea.Cmd {
	return m.animation.Init()
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m.quitting = true
		return m, tea.Quit
	case tea.QuitMsg:
		m.quitting = true
		return m, tea.Quit
	default:
		a, cmd := m.animation.Update(msg)
		m.animation = a.(anim.Anim)
		return m, cmd
	}
}

func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}
	return m.animation.View()
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(ctx context.Context, message string) *Spinner {
	t := styles.CurrentTheme()
	prog := tea.NewProgram(
		spinnerModel{
			animation: anim.New(anim.Settings{
				Size:        10,
				Label:       message,
				LabelColor:  t.FgBase,
				GradColorA:  t.Primary,
				GradColorB:  t.Secondary,
				CycleColors: true,
			}),
		},
		tea.WithOutput(os.Stderr),
		tea.WithContext(ctx),
		tea.WithoutCatchPanics(),
	)

	return &Spinner{
		prog: prog,
		done: make(chan struct{}, 1),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	go func() {
		_, err := s.prog.Run()
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "Error running spinner: %v\n", err)
		}
		close(s.done)
	}()
}

// Stop ends the spinner animation
func (s *Spinner) Stop() {
	s.prog.Quit()
	<-s.done
}
