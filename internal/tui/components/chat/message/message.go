package message

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type MessageModel interface {
	util.Model
	layout.Sizeable
	layout.Focusable
}

type model struct {
	width               int
	focused             bool
	message             message.Message
	lastUserMessageTime time.Time
	toolResults         []message.ToolResult

	// Only used for the agent tool
	childSessionMessages []message.Message
}

type messageOption func(*model)

func WithToolResults(toolResults []message.ToolResult) messageOption {
	return func(m *model) {
		m.toolResults = toolResults
	}
}

func WithLastUserMessageTime(t time.Time) messageOption {
	return func(m *model) {
		m.lastUserMessageTime = t
	}
}

func WithChildSessionMessages(messages []message.Message) messageOption {
	return func(m *model) {
		m.childSessionMessages = messages
	}
}

func New(msg message.Message, opts ...messageOption) MessageModel {
	m := &model{
		message: msg,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *model) View() string {
	switch m.message.Role {
	case message.User:
		return m.renderUserMessage()
	default:
		return ""
	}
}

func (m *model) renderUserMessage() string {
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

func (m *model) markdownContent() string {
	r := styles.GetMarkdownRenderer(m.textWidth())
	rendered, _ := r.Render(m.message.Content().String())
	return strings.TrimSuffix(rendered, "\n")
}

func (m *model) textWidth() int {
	return m.width - 1 // take into account the border
}

func (msg *model) style() lipgloss.Style {
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
		borderColor = t.BorderDim()
	}

	return styles.BaseStyle().
		BorderLeft(true).
		Foreground(t.TextMuted()).
		BorderForeground(borderColor).
		BorderStyle(borderStyle)
}

// Blur implements MessageModel.
func (m *model) Blur() tea.Cmd {
	m.focused = false
	return nil
}

// Focus implements MessageModel.
func (m *model) Focus() tea.Cmd {
	m.focused = true
	return nil
}

// IsFocused implements MessageModel.
func (m *model) IsFocused() bool {
	return m.focused
}

func (m *model) GetSize() (int, int) {
	return m.width, 0
}

func (m *model) SetSize(width int, height int) tea.Cmd {
	m.width = width
	return nil
}
