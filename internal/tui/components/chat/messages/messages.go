package messages

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/llm/models"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

// MessageCmp defines the interface for message components in the chat interface.
// It combines standard UI model interfaces with message-specific functionality.
type MessageCmp interface {
	util.Model                   // Basic Bubble Tea model interface
	layout.Sizeable              // Width/height management
	layout.Focusable             // Focus state management
	GetMessage() message.Message // Access to underlying message data
	Spinning() bool              // Animation state for loading messages
}

// messageCmp implements the MessageCmp interface for displaying chat messages.
// It handles rendering of user and assistant messages with proper styling,
// animations, and state management.
type messageCmp struct {
	width   int  // Component width for text wrapping
	focused bool // Focus state for border styling

	// Core message data and state
	message             message.Message // The underlying message content
	spinning            bool            // Whether to show loading animation
	anim                util.Model      // Animation component for loading states
	lastUserMessageTime time.Time       // Used for calculating response duration
}

// MessageOption provides functional options for configuring message components
type MessageOption func(*messageCmp)

// WithLastUserMessageTime sets the timestamp of the last user message
// for calculating assistant response duration
func WithLastUserMessageTime(t time.Time) MessageOption {
	return func(m *messageCmp) {
		m.lastUserMessageTime = t
	}
}

// NewMessageCmp creates a new message component with the given message and options
func NewMessageCmp(msg message.Message, opts ...MessageOption) MessageCmp {
	m := &messageCmp{
		message: msg,
		anim:    anim.New(15, ""),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Init initializes the message component and starts animations if needed.
// Returns a command to start the animation for spinning messages.
func (m *messageCmp) Init() tea.Cmd {
	m.spinning = m.shouldSpin()
	if m.spinning {
		return m.anim.Init()
	}
	return nil
}

// Update handles incoming messages and updates the component state.
// Manages animation updates for spinning messages and stops animation when appropriate.
func (m *messageCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case anim.ColorCycleMsg, anim.StepCharsMsg, spinner.TickMsg:
		m.spinning = m.shouldSpin()
		if m.spinning {
			u, cmd := m.anim.Update(msg)
			m.anim = u.(util.Model)
			return m, cmd
		}
	}
	return m, nil
}

// View renders the message component based on its current state.
// Returns different views for spinning, user, and assistant messages.
func (m *messageCmp) View() tea.View {
	if m.spinning {
		return tea.NewView(m.style().PaddingLeft(1).Render(m.anim.View().String()))
	}
	if m.message.ID != "" {
		// this is a user or assistant message
		switch m.message.Role {
		case message.User:
			return tea.NewView(m.renderUserMessage())
		default:
			return tea.NewView(m.renderAssistantMessage())
		}
	}
	return tea.NewView(m.style().Render("No message content"))
}

// GetMessage returns the underlying message data
func (m *messageCmp) GetMessage() message.Message {
	return m.message
}

// textWidth calculates the available width for text content,
// accounting for borders and padding
func (m *messageCmp) textWidth() int {
	return m.width - 2 // take into account the border and/or padding
}

// style returns the lipgloss style for the message component.
// Applies different border colors and styles based on message role and focus state.
func (msg *messageCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	borderStyle := lipgloss.NormalBorder()
	if msg.focused {
		borderStyle = lipgloss.ThickBorder()
	}

	style := t.S().Text
	if msg.message.Role == message.User {
		style = style.PaddingLeft(1).BorderLeft(true).BorderStyle(borderStyle).BorderForeground(t.Primary)
	} else {
		if msg.focused {
			style = style.PaddingLeft(1).BorderLeft(true).BorderStyle(borderStyle).BorderForeground(t.GreenDark)
		} else {
			style = style.PaddingLeft(2)
		}
	}
	return style
}

// renderAssistantMessage renders assistant messages with optional footer information.
// Shows model name, response time, and finish reason when the message is complete.
func (m *messageCmp) renderAssistantMessage() string {
	t := styles.CurrentTheme()
	parts := []string{
		m.markdownContent(),
	}

	finished := m.message.IsFinished()
	finishData := m.message.FinishPart()
	// Only show the footer if the message is not a tool call
	if finished && finishData.Reason != message.FinishReasonToolUse {
		infoMsg := ""
		switch finishData.Reason {
		case message.FinishReasonEndTurn:
			finishTime := time.Unix(finishData.Time, 0)
			duration := finishTime.Sub(m.lastUserMessageTime)
			infoMsg = duration.String()
		case message.FinishReasonCanceled:
			infoMsg = "canceled"
		case message.FinishReasonError:
			infoMsg = "error"
		case message.FinishReasonPermissionDenied:
			infoMsg = "permission denied"
		}
		assistant := t.S().Muted.Render(fmt.Sprintf("⬡ %s (%s)", models.SupportedModels[m.message.Model].Name, infoMsg))
		parts = append(parts, core.Section(assistant, m.textWidth()))
	}

	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return m.style().Render(joined)
}

// renderUserMessage renders user messages with file attachments.
// Displays message content and any attached files with appropriate icons.
func (m *messageCmp) renderUserMessage() string {
	t := styles.CurrentTheme()
	parts := []string{
		m.markdownContent(),
	}
	attachmentStyles := t.S().Text.
		MarginLeft(1).
		Background(t.BgSubtle)
	attachments := []string{}
	for _, attachment := range m.message.BinaryContent() {
		file := filepath.Base(attachment.Path)
		var filename string
		if len(file) > 10 {
			filename = fmt.Sprintf(" %s %s... ", styles.DocumentIcon, file[0:7])
		} else {
			filename = fmt.Sprintf(" %s %s ", styles.DocumentIcon, file)
		}
		attachments = append(attachments, attachmentStyles.Render(filename))
	}
	if len(attachments) > 0 {
		parts = append(parts, "", strings.Join(attachments, ""))
	}
	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return m.style().MarginBottom(1).Render(joined)
}

// toMarkdown converts text content to rendered markdown using the configured renderer
func (m *messageCmp) toMarkdown(content string) string {
	r := styles.GetMarkdownRenderer(m.textWidth())
	rendered, _ := r.Render(content)
	return strings.TrimSuffix(rendered, "\n")
}

// markdownContent processes the message content and handles special states.
// Returns appropriate content for thinking, finished, and error states.
func (m *messageCmp) markdownContent() string {
	content := m.message.Content().String()
	if m.message.Role == message.Assistant {
		thinking := m.message.IsThinking()
		finished := m.message.IsFinished()
		finishedData := m.message.FinishPart()
		if thinking {
			// Handle the thinking state
			// TODO: maybe add the thinking content if available later.
			content = fmt.Sprintf("**%s %s**", styles.LoadingIcon, "Thinking...")
		} else if finished && content == "" && finishedData.Reason == message.FinishReasonEndTurn {
			// Sometimes the LLMs respond with no content when they think the previous tool result
			//  provides the requested question
			content = "*Finished without output*"
		} else if finished && content == "" && finishedData.Reason == message.FinishReasonCanceled {
			content = "*Canceled*"
		}
	}
	return m.toMarkdown(content)
}

// shouldSpin determines whether the message should show a loading animation.
// Only assistant messages without content that aren't finished should spin.
func (m *messageCmp) shouldSpin() bool {
	if m.message.Role != message.Assistant {
		return false
	}

	if m.message.IsFinished() {
		return false
	}

	if m.message.Content().Text != "" {
		return false
	}
	return true
}

// Focus management methods

// Blur removes focus from the message component
func (m *messageCmp) Blur() tea.Cmd {
	m.focused = false
	return nil
}

// Focus sets focus on the message component
func (m *messageCmp) Focus() tea.Cmd {
	m.focused = true
	return nil
}

// IsFocused returns whether the message component is currently focused
func (m *messageCmp) IsFocused() bool {
	return m.focused
}

// Size management methods

// GetSize returns the current dimensions of the message component
func (m *messageCmp) GetSize() (int, int) {
	return m.width, 0
}

// SetSize updates the width of the message component for text wrapping
func (m *messageCmp) SetSize(width int, height int) tea.Cmd {
	// For better readability, we limit the width to a maximum of 120 characters
	m.width = min(width, 120)
	return nil
}

// Spinning returns whether the message is currently showing a loading animation
func (m *messageCmp) Spinning() bool {
	return m.spinning
}
