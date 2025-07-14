package messages

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
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
	message  message.Message // The underlying message content
	spinning bool            // Whether to show loading animation
	anim     util.Model      // Animation component for loading states
}

var focusedMessageBorder = lipgloss.Border{
	Left: "▌",
}

// NewMessageCmp creates a new message component with the given message and options
func NewMessageCmp(msg message.Message) MessageCmp {
	t := styles.CurrentTheme()
	m := &messageCmp{
		message: msg,
		anim: anim.New(anim.Settings{
			Size:        15,
			GradColorA:  t.Primary,
			GradColorB:  t.Secondary,
			CycleColors: true,
		}),
	}
	return m
}

// Init initializes the message component and starts animations if needed.
// Returns a command to start the animation for spinning messages.
func (m *messageCmp) Init() tea.Cmd {
	m.spinning = m.shouldSpin()
	return m.anim.Init()
}

// Update handles incoming messages and updates the component state.
// Manages animation updates for spinning messages and stops animation when appropriate.
func (m *messageCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case anim.StepMsg:
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
func (m *messageCmp) View() string {
	if m.spinning {
		return m.style().PaddingLeft(1).Render(m.anim.View())
	}
	if m.message.ID != "" {
		// this is a user or assistant message
		switch m.message.Role {
		case message.User:
			return m.renderUserMessage()
		default:
			return m.renderAssistantMessage()
		}
	}
	return m.style().Render("No message content")
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
		borderStyle = focusedMessageBorder
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
	parts := []string{
		m.markdownContent(),
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
	return m.style().Render(joined)
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
	t := styles.CurrentTheme()
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
			content = ""
		} else if finished && content == "" && finishedData.Reason == message.FinishReasonCanceled {
			content = "*Canceled*"
		} else if finished && content == "" && finishedData.Reason == message.FinishReasonError {
			errTag := t.S().Base.Padding(0, 1).Background(t.Red).Foreground(t.White).Render("ERROR")
			truncated := ansi.Truncate(finishedData.Message, m.textWidth()-2-lipgloss.Width(errTag), "...")
			title := fmt.Sprintf("%s %s", errTag, t.S().Base.Foreground(t.FgHalfMuted).Render(truncated))
			details := t.S().Base.Foreground(t.FgSubtle).Width(m.textWidth() - 2).Render(finishedData.Details)
			return fmt.Sprintf("%s\n\n%s", title, details)

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

type AssistantSection interface {
	util.Model
	layout.Sizeable
	list.SectionHeader
}
type assistantSectionModel struct {
	width               int
	message             message.Message
	lastUserMessageTime time.Time
}

func NewAssistantSection(message message.Message, lastUserMessageTime time.Time) AssistantSection {
	return &assistantSectionModel{
		width:               0,
		message:             message,
		lastUserMessageTime: lastUserMessageTime,
	}
}

func (m *assistantSectionModel) Init() tea.Cmd {
	return nil
}

func (m *assistantSectionModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *assistantSectionModel) View() string {
	t := styles.CurrentTheme()
	finishData := m.message.FinishPart()
	finishTime := time.Unix(finishData.Time, 0)
	duration := finishTime.Sub(m.lastUserMessageTime)
	infoMsg := t.S().Subtle.Render(duration.String())
	icon := t.S().Subtle.Render(styles.ModelIcon)
	model := config.Get().GetModel(m.message.Provider, m.message.Model)
	if model == nil {
		// This means the model is not configured anymore
		model = &provider.Model{
			Model: "Unknown Model",
		}
	}
	modelFormatted := t.S().Muted.Render(model.Model)
	assistant := fmt.Sprintf("%s %s %s", icon, modelFormatted, infoMsg)
	return t.S().Base.PaddingLeft(2).Render(
		core.Section(assistant, m.width-2),
	)
}

func (m *assistantSectionModel) GetSize() (int, int) {
	return m.width, 1
}

func (m *assistantSectionModel) SetSize(width int, height int) tea.Cmd {
	m.width = width
	return nil
}

func (m *assistantSectionModel) IsSectionHeader() bool {
	return true
}
