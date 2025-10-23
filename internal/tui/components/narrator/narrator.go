package narrator

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/narrator"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

// Model represents the narrator UI component
type Model struct {
	width        int
	height       int
	agent        *narrator.Agent
	explainer    *narrator.Explainer
	messages     []string
	maxMessages  int
	isExplaining bool
	streamChan   <-chan string
	lastUpdate   time.Time
}

// New creates a new narrator model
func New(width, height int, agent *narrator.Agent, explainer *narrator.Explainer) *Model {
	return &Model{
		width:        width,
		height:       height,
		agent:        agent,
		explainer:    explainer,
		messages:     []string{},
		maxMessages:  50,
		isExplaining: false,
		lastUpdate:   time.Now(),
	}
}

// Init initializes the narrator model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the narrator state
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case ExplainMsg:
		m.explain(msg.Context)
	case StreamStartedMsg:
		m.isExplaining = true
		m.streamChan = msg.StreamChan
	case StreamChunkMsg:
		m.addMessage(msg.Chunk)
	case StreamEndedMsg:
		m.isExplaining = false
		m.streamChan = nil
	case ClearMsg:
		m.messages = []string{}
	}
	return m, nil
}

// View renders the narrator component
func (m *Model) View() string {
	theme := styles.CurrentTheme()

	// Build the content
	var content strings.Builder

	// Header
	header := theme.S().Base.Padding(0, 1).Render("🤖 AI Narrator")
	content.WriteString(header)
	content.WriteString("\n")

	// Status indicator
	status := m.renderStatus()
	content.WriteString(status)
	content.WriteString("\n")

	// Messages (scrollable area)
	messages := m.renderMessages()
	content.WriteString(messages)

	// Apply border and style
	return theme.S().Base.
		Width(m.width).
		Height(m.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus).
		Render(content.String())
}

// renderStatus renders the status indicator
func (m *Model) renderStatus() string {
	theme := styles.CurrentTheme()

	var statusText string
	var statusStyle lipgloss.Style

	if m.isExplaining {
		statusText = "Thinking... ⚡"
		statusStyle = theme.S().Base.Foreground(theme.Primary).Bold(true)
	} else if m.agent.IsEnabled() {
		statusText = "Waiting for events... ⚪"
		statusStyle = theme.S().Base.Foreground(theme.FgHalfMuted)
	} else {
		statusText = "AI narrator disabled ⚫"
		statusStyle = theme.S().Base.Foreground(theme.FgHalfMuted).Italic(true)
	}

	return theme.S().Base.Padding(0, 1).Render(statusStyle.Render(statusText))
}

// renderMessages renders the message history
func (m *Model) renderMessages() string {
	theme := styles.CurrentTheme()

	if len(m.messages) == 0 {
		return theme.S().Base.
			Height(m.height - 4).
			AlignVertical(lipgloss.Center).
			AlignHorizontal(lipgloss.Center).
			Foreground(theme.FgHalfMuted).
			Render("No explanations yet")
	}

	// Show last messages, newest at bottom
	visibleHeight := m.height - 4
	startIdx := max(0, len(m.messages)-visibleHeight)

	var visibleMessages []string
	for i := startIdx; i < len(m.messages); i++ {
		visibleMessages = append(visibleMessages, m.messages[i])
	}

	return strings.Join(visibleMessages, "\n")
}

// explain triggers an explanation for the given context
func (m *Model) explain(context *narrator.Context) {
	if m.agent == nil || m.explainer == nil {
		return
	}

	// Build prompt
	prompt := m.explainer.BuildPrompt(context)

	// Start streaming
	streamChan, err := m.agent.ExplainStream(nil, prompt)
	if err != nil {
		m.addMessage("Error: " + err.Error())
		return
	}

	m.streamChan = streamChan
	m.isExplaining = true

	// Add a message that we're starting
	m.addMessage("🔍 Analyzing...")
}

// addMessage adds a message to the history
func (m *Model) addMessage(message string) {
	timestamp := time.Now().Format("15:04:05")
	formattedMessage := fmt.Sprintf("[%s] %s", timestamp, message)

	m.messages = append(m.messages, formattedMessage)
	m.lastUpdate = time.Now()

	// Keep only the most recent messages
	if len(m.messages) > m.maxMessages {
		m.messages = m.messages[len(m.messages)-m.maxMessages:]
	}
}

// Messages represents the message history
type Messages []string

// ExplainMsg triggers an explanation
type ExplainMsg struct {
	Context *narrator.Context
}

// StreamStartedMsg indicates that streaming has started
type StreamStartedMsg struct {
	StreamChan <-chan string
}

// StreamChunkMsg represents a chunk of streaming response
type StreamChunkMsg struct {
	Chunk string
}

// StreamEndedMsg indicates that streaming has ended
type StreamEndedMsg struct{}

// ClearMsg clears the message history
type ClearMsg struct{}

// Helper function to get the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
