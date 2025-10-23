package status

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

// Model represents the status bar component
type Model struct {
	width       int
	height      int
	spinner     spinner.Model
	messages    []string
	showTime    bool
	showSpinner bool
	lastUpdate  time.Time
}

// New creates a new status bar model
func New(width, height int) *Model {
	s := spinner.New()
	s.Spinner = spinner.Line

	return &Model{
		width:       width,
		height:      height,
		spinner:     s,
		messages:    []string{},
		showTime:    true,
		showSpinner: false,
		lastUpdate:  time.Now(),
	}
}

// Init initializes the status bar model
func (m *Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages and updates the status bar state
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case StatusMsg:
		m.addMessage(msg.Text)
	case SetSpinnerMsg:
		m.showSpinner = msg.Show
		if msg.Show {
			return m, m.spinner.Tick
		}
	case ClearStatusMsg:
		m.messages = []string{}
	}
	return m, nil
}

// View renders the status bar
func (m *Model) View() string {
	theme := styles.CurrentTheme()

	// Build the status content
	var statusContent strings.Builder

	// Left side: spinner (if active) and messages
	leftContent := m.renderLeftContent()
	statusContent.WriteString(leftContent)

	// Right side: time and other info
	rightContent := m.renderRightContent()
	if rightContent != "" {
		statusContent.WriteString(strings.Repeat(" ", max(0, m.width-lipgloss.Width(leftContent)-lipgloss.Width(rightContent))))
		statusContent.WriteString(rightContent)
	}

	// Apply style and return
	return theme.S().Base.
		Width(m.width).
		Height(m.height).
		Foreground(theme.FgBase).
		Background(theme.BgBase).
		Render(statusContent.String())
}

// renderLeftContent renders the left side of the status bar
func (m *Model) renderLeftContent() string {
	var content strings.Builder

	// Spinner (if active)
	if m.showSpinner {
		spinnerView := m.spinner.View()
		content.WriteString(spinnerView)
		content.WriteString(" ")
	}

	// Messages
	if len(m.messages) > 0 {
		// Show the most recent message
		content.WriteString(m.messages[len(m.messages)-1])
	} else {
		content.WriteString("Ready")
	}

	return content.String()
}

// renderRightContent renders the right side of the status bar
func (m *Model) renderRightContent() string {
	var content strings.Builder

	// Current time
	if m.showTime {
		now := time.Now().Format("2006-01-02 15:04:05")
		content.WriteString(now)
	}

	return content.String()
}

// addMessage adds a message to the status bar
func (m *Model) addMessage(text string) {
	m.messages = append(m.messages, text)
	m.lastUpdate = time.Now()

	// Keep only the last 10 messages
	if len(m.messages) > 10 {
		m.messages = m.messages[len(m.messages)-10:]
	}
}

// SetSpinner enables or disables the spinner
func (m *Model) SetSpinner(show bool) {
	m.showSpinner = show
}

// SetShowTime enables or disables time display
func (m *Model) SetShowTime(show bool) {
	m.showTime = show
}

// ClearMessages clears all status messages
func (m *Model) ClearMessages() {
	m.messages = []string{}
}

// Message types
type StatusMsg struct {
	Text string
}

type SetSpinnerMsg struct {
	Show bool
}

type ClearStatusMsg struct{}

// Helper function to get the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
