package format

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Spinner wraps the bubbles spinner for non-interactive mode
type Spinner struct {
	model  spinner.Model
	done   chan struct{}
	prog   *tea.Program
	ctx    context.Context
	cancel context.CancelFunc
}

// spinnerModel is the tea.Model for the spinner
type spinnerModel struct {
	spinner  spinner.Model
	message  string
	quitting bool
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.quitting = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case quitMsg:
		m.quitting = true
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

// quitMsg is sent when we want to quit the spinner
type quitMsg struct{}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = s.Style.Foreground(s.Style.GetForeground())

	ctx, cancel := context.WithCancel(context.Background())

	model := spinnerModel{
		spinner: s,
		message: message,
	}

	prog := tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithoutCatchPanics())

	return &Spinner{
		model:  s,
		done:   make(chan struct{}),
		prog:   prog,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	go func() {
		defer close(s.done)
		go func() {
			<-s.ctx.Done()
			s.prog.Send(quitMsg{})
		}()
		_, err := s.prog.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running spinner: %v\n", err)
		}
	}()
}

// Stop ends the spinner animation
func (s *Spinner) Stop() {
	s.cancel()
	<-s.done
}
