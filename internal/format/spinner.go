package format

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/x/ansi"
)

// Spinner wraps the bubbles spinner for non-interactive mode
type Spinner struct {
	done chan struct{}
	prog *tea.Program
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(ctx context.Context, message string) *Spinner {
	t := styles.CurrentTheme()
	model := anim.New(anim.Settings{
		Size:        10,
		Label:       message,
		LabelColor:  t.FgBase,
		GradColorA:  t.Primary,
		GradColorB:  t.Secondary,
		CycleColors: true,
	})

	prog := tea.NewProgram(
		model,
		tea.WithInput(nil),
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
		// ensures line is cleared
		fmt.Fprint(os.Stderr, ansi.EraseEntireLine)
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, tea.ErrInterrupted) {
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
