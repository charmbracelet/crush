package messages

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/llm/models"

	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/tui/components/anim"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type MessageCmp interface {
	util.Model
	layout.Sizeable
	layout.Focusable
	GetMessage() message.Message
	Spinning() bool
}

type messageCmp struct {
	width   int
	focused bool

	// Used for agent and user messages
	message             message.Message
	spinning            bool
	anim                util.Model
	lastUserMessageTime time.Time
}
type MessageOption func(*messageCmp)

func WithLastUserMessageTime(t time.Time) MessageOption {
	return func(m *messageCmp) {
		m.lastUserMessageTime = t
	}
}

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

func (m *messageCmp) Init() tea.Cmd {
	m.spinning = m.shouldSpin()
	if m.spinning {
		return m.anim.Init()
	}
	return nil
}

func (m *messageCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	u, cmd := m.anim.Update(msg)
	m.anim = u.(util.Model)
	return m, cmd
}

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
	return "Unknown Message"
}

// GetMessage implements MessageCmp.
func (m *messageCmp) GetMessage() message.Message {
	return m.message
}

func (m *messageCmp) textWidth() int {
	return m.width - 1 // take into account the border
}

func (msg *messageCmp) style() lipgloss.Style {
	t := theme.CurrentTheme()
	var borderColor color.Color
	borderStyle := lipgloss.NormalBorder()
	if msg.focused {
		borderStyle = lipgloss.DoubleBorder()
	}

	switch msg.message.Role {
	case message.User:
		borderColor = t.Secondary()
	case message.Assistant:
		borderColor = t.Primary()
	default:
		// Tool call
		borderColor = t.TextMuted()
	}

	return styles.BaseStyle().
		BorderLeft(true).
		Foreground(t.TextMuted()).
		BorderForeground(borderColor).
		BorderStyle(borderStyle)
}

func (m *messageCmp) renderAssistantMessage() string {
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
		parts = append(parts, fmt.Sprintf(" %s (%s)", models.SupportedModels[m.message.Model].Name, infoMsg))
	}

	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return m.style().Render(joined)
}

func (m *messageCmp) renderUserMessage() string {
	t := theme.CurrentTheme()
	parts := []string{
		m.markdownContent(),
	}
	attachmentStyles := styles.BaseStyle().
		MarginLeft(1).
		Background(t.BackgroundSecondary()).
		Foreground(t.Text())
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

func (m *messageCmp) toMarkdown(content string) string {
	r := styles.GetMarkdownRenderer(m.textWidth())
	rendered, _ := r.Render(content)
	return strings.TrimSuffix(rendered, "\n")
}

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

// Blur implements MessageModel.
func (m *messageCmp) Blur() tea.Cmd {
	m.focused = false
	return nil
}

// Focus implements MessageModel.
func (m *messageCmp) Focus() tea.Cmd {
	m.focused = true
	return nil
}

// IsFocused implements MessageModel.
func (m *messageCmp) IsFocused() bool {
	return m.focused
}

func (m *messageCmp) GetSize() (int, int) {
	return m.width, 0
}

func (m *messageCmp) SetSize(width int, height int) tea.Cmd {
	m.width = width
	return nil
}

// Spinning implements MessageCmp.
func (m *messageCmp) Spinning() bool {
	return m.spinning
}
