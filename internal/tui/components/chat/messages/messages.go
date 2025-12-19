package messages

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/ordered"
	"github.com/google/uuid"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

// CopyKey is the key binding for copying message content to the clipboard.
var CopyKey = key.NewBinding(key.WithKeys("c", "y", "C", "Y"), key.WithHelp("c/y", "copy"))

// ClearSelectionKey is the key binding for clearing the current selection in the chat interface.
var ClearSelectionKey = key.NewBinding(key.WithKeys("esc", "alt+esc"), key.WithHelp("esc", "clear selection"))

// MessageCmp defines the interface for message components in the chat interface.
// It combines standard UI model interfaces with message-specific functionality.
type MessageCmp interface {
	util.Model                      // Basic Bubble util.Model interface
	layout.Sizeable                 // Width/height management
	layout.Focusable                // Focus state management
	GetMessage() message.Message    // Access to underlying message data
	SetMessage(msg message.Message) // Update the message content
	Spinning() bool                 // Animation state for loading messages
	ID() string
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
	anim     *anim.Anim      // Animation component for loading states

	// Thinking viewport for displaying reasoning content
	thinkingViewport viewport.Model
}

var focusedMessageBorder = lipgloss.Border{
	Left: "▌",
}

// NewMessageCmp creates a new message component with the given message and options
func NewMessageCmp(msg message.Message) MessageCmp {
	t := styles.CurrentTheme()

	thinkingViewport := viewport.New()
	thinkingViewport.SetHeight(1)
	thinkingViewport.KeyMap = viewport.KeyMap{}

	m := &messageCmp{
		message: msg,
		anim: anim.New(anim.Settings{
			Size:        15,
			GradColorA:  t.Primary,
			GradColorB:  t.Secondary,
			CycleColors: true,
		}),
		thinkingViewport: thinkingViewport,
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
func (m *messageCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case anim.StepMsg:
		m.spinning = m.shouldSpin()
		if m.spinning {
			u, cmd := m.anim.Update(msg)
			m.anim = u.(*anim.Anim)
			return m, cmd
		}
	case tea.KeyPressMsg:
		if key.Matches(msg, CopyKey) {
			return m, tea.Sequence(
				tea.SetClipboard(m.message.Content().Text),
				func() tea.Msg {
					_ = clipboard.WriteAll(m.message.Content().Text)
					return nil
				},
				util.ReportInfo("Message copied to clipboard"),
			)
		}
	}
	return m, nil
}

// View renders the message component based on its current state.
// Returns different views for spinning, user, and assistant messages.
func (m *messageCmp) View() string {
	if m.spinning && m.message.ReasoningContent().Thinking == "" {
		if m.message.IsSummaryMessage {
			m.anim.SetLabel("Summarizing")
		}
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

func (m *messageCmp) SetMessage(msg message.Message) {
	m.message = msg
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
	t := styles.CurrentTheme()
	parts := []string{}
	content := strings.TrimSpace(m.message.Content().String())
	thinking := m.message.IsThinking()
	thinkingContent := strings.TrimSpace(m.message.ReasoningContent().Thinking)
	finished := m.message.IsFinished()
	finishedData := m.message.FinishPart()

	if thinking || thinkingContent != "" {
		m.anim.SetLabel("Thinking")
		thinkingContent = m.renderThinkingContent()
	} else if finished && content == "" && finishedData.Reason == message.FinishReasonEndTurn {
		// Don't render empty assistant messages with EndTurn
		return ""
	} else if finished && content == "" && finishedData.Reason == message.FinishReasonCanceled {
		content = "*Canceled*"
	} else if finished && content == "" && finishedData.Reason == message.FinishReasonError {
		errTag := t.S().Base.Padding(0, 1).Background(t.Red).Foreground(t.White).Render("ERROR")
		truncated := ansi.Truncate(finishedData.Message, m.textWidth()-2-lipgloss.Width(errTag), "...")
		title := fmt.Sprintf("%s %s", errTag, t.S().Base.Foreground(t.FgHalfMuted).Render(truncated))
		details := t.S().Base.Foreground(t.FgSubtle).Width(m.textWidth() - 2).Render(finishedData.Details)
		errorContent := fmt.Sprintf("%s\n\n%s", title, details)
		return m.style().Render(errorContent)
	}

	if thinkingContent != "" {
		parts = append(parts, thinkingContent)
	}

	if content != "" {
		if thinkingContent != "" {
			parts = append(parts, "")
		}
		parts = append(parts, m.toMarkdown(content))
	}

	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return m.style().Render(joined)
}

// renderUserMessage renders user messages with file attachments. It displays
// message content and any attached files with appropriate icons.
func (m *messageCmp) renderUserMessage() string {
	t := styles.CurrentTheme()
	parts := []string{
		m.toMarkdown(m.message.Content().String()),
	}

	attachmentStyle := t.S().Base.
		Padding(0, 1).
		MarginRight(1).
		Background(t.FgMuted).
		Foreground(t.FgBase).
		Render
	iconStyle := t.S().Base.
		Foreground(t.BgSubtle).
		Background(t.Green).
		Padding(0, 1).
		Bold(true).
		Render

	attachments := make([]string, len(m.message.BinaryContent()))
	for i, attachment := range m.message.BinaryContent() {
		const maxFilenameWidth = 10
		filename := ansi.Truncate(filepath.Base(attachment.Path), 10, "...")
		icon := styles.ImageIcon
		if strings.HasPrefix(attachment.MIMEType, "text/") {
			icon = styles.TextIcon
		}
		attachments[i] = lipgloss.JoinHorizontal(
			lipgloss.Left,
			iconStyle(icon),
			attachmentStyle(filename),
		)
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

func (m *messageCmp) renderThinkingContent() string {
	t := styles.CurrentTheme()
	reasoningContent := m.message.ReasoningContent()
	if strings.TrimSpace(reasoningContent.Thinking) == "" {
		return ""
	}

	width := m.textWidth() - 2
	width = min(width, 120)

	renderer := styles.GetPlainMarkdownRenderer(width - 1)
	rendered, err := renderer.Render(reasoningContent.Thinking)
	if err != nil {
		lines := strings.Split(reasoningContent.Thinking, "\n")
		var content strings.Builder
		lineStyle := t.S().Subtle.Background(t.BgBaseLighter)
		for i, line := range lines {
			if line == "" {
				continue
			}
			content.WriteString(lineStyle.Width(width).Render(line))
			if i < len(lines)-1 {
				content.WriteString("\n")
			}
		}
		rendered = content.String()
	}

	fullContent := strings.TrimSpace(rendered)
	height := ordered.Clamp(lipgloss.Height(fullContent), 1, 10)
	m.thinkingViewport.SetHeight(height)
	m.thinkingViewport.SetWidth(m.textWidth())
	m.thinkingViewport.SetContent(fullContent)
	m.thinkingViewport.GotoBottom()
	finishReason := m.message.FinishPart()
	var footer string
	if reasoningContent.StartedAt > 0 {
		duration := m.message.ThinkingDuration()
		if reasoningContent.FinishedAt > 0 {
			m.anim.SetLabel("")
			opts := core.StatusOpts{
				Title:       "Thought for",
				Description: duration.String(),
			}
			if duration.String() != "0s" {
				footer = t.S().Base.PaddingLeft(1).Render(core.Status(opts, m.textWidth()-1))
			}
		} else if finishReason != nil && finishReason.Reason == message.FinishReasonCanceled {
			footer = t.S().Base.PaddingLeft(1).Render(m.toMarkdown("*Canceled*"))
		} else {
			footer = m.anim.View()
		}
	}
	lineStyle := t.S().Subtle.Background(t.BgBaseLighter)
	result := lineStyle.Width(m.textWidth()).Padding(0, 1, 0, 0).Render(m.thinkingViewport.View())
	if footer != "" {
		result += "\n\n" + footer
	}
	return result
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

	if strings.TrimSpace(m.message.Content().Text) != "" {
		return false
	}
	if len(m.message.ToolCalls()) > 0 {
		return false
	}
	return true
}

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
	m.width = ordered.Clamp(width, 1, 120)
	m.thinkingViewport.SetWidth(m.width - 4)
	return nil
}

// Spinning returns whether the message is currently showing a loading animation
func (m *messageCmp) Spinning() bool {
	return m.spinning
}

type AssistantSection interface {
	list.Item
	layout.Sizeable
}
type assistantSectionModel struct {
	width               int
	id                  string
	message             message.Message
	lastUserMessageTime time.Time
}

// ID implements AssistantSection.
func (m *assistantSectionModel) ID() string {
	return m.id
}

func NewAssistantSection(message message.Message, lastUserMessageTime time.Time) AssistantSection {
	return &assistantSectionModel{
		width:               0,
		id:                  uuid.NewString(),
		message:             message,
		lastUserMessageTime: lastUserMessageTime,
	}
}

func (m *assistantSectionModel) Init() tea.Cmd {
	return nil
}

func (m *assistantSectionModel) Update(tea.Msg) (util.Model, tea.Cmd) {
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
		model = &catwalk.Model{
			Name: "Unknown Model",
		}
	}
	modelFormatted := t.S().Muted.Render(model.Name)
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

func (m *messageCmp) ID() string {
	return m.message.ID
}
