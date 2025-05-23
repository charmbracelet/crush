package messages

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/tui/components/anim"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type ToolCallCmp interface {
	util.Model
	layout.Sizeable
	layout.Focusable
	GetToolCall() message.ToolCall
	GetToolResult() message.ToolResult
	SetToolResult(message.ToolResult)
	SetToolCall(message.ToolCall)
	SetCancelled()
	ParentMessageId() string
	Spinning() bool
}

type toolCallCmp struct {
	width   int
	focused bool

	parentMessageId string
	call            message.ToolCall
	result          message.ToolResult
	cancelled       bool

	spinning bool
	anim     util.Model
}

type ToolCallOption func(*toolCallCmp)

func WithToolCallCancelled() ToolCallOption {
	return func(m *toolCallCmp) {
		m.cancelled = true
	}
}

func WithToolCallResult(result message.ToolResult) ToolCallOption {
	return func(m *toolCallCmp) {
		m.result = result
	}
}

func NewToolCallCmp(parentMessageId string, tc message.ToolCall, opts ...ToolCallOption) ToolCallCmp {
	m := &toolCallCmp{
		call:            tc,
		parentMessageId: parentMessageId,
		anim:            anim.New(15, "Working"),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *toolCallCmp) Init() tea.Cmd {
	m.spinning = m.shouldSpin()
	logging.Info("Initializing tool call spinner", "tool_call", m.call.Name, "spinning", m.spinning)
	if m.spinning {
		return m.anim.Init()
	}
	return nil
}

func (m *toolCallCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	logging.Debug("Tool call update", "msg", msg)
	switch msg := msg.(type) {
	case anim.ColorCycleMsg, anim.StepCharsMsg:
		if m.spinning {
			u, cmd := m.anim.Update(msg)
			m.anim = u.(util.Model)
			return m, cmd
		}
	}
	return m, nil
}

func (m *toolCallCmp) View() string {
	box := m.style()

	if !m.call.Finished && !m.cancelled {
		return box.PaddingLeft(1).Render(m.renderPending())
	}

	r := registry.lookup(m.call.Name)
	return box.PaddingLeft(1).Render(r.Render(m))
}

// SetCancelled implements ToolCallCmp.
func (m *toolCallCmp) SetCancelled() {
	m.cancelled = true
}

// SetToolCall implements ToolCallCmp.
func (m *toolCallCmp) SetToolCall(call message.ToolCall) {
	m.call = call
	if m.call.Finished {
		m.spinning = false
	}
}

// ParentMessageId implements ToolCallCmp.
func (m *toolCallCmp) ParentMessageId() string {
	return m.parentMessageId
}

// SetToolResult implements ToolCallCmp.
func (m *toolCallCmp) SetToolResult(result message.ToolResult) {
	m.result = result
	m.spinning = false
}

// GetToolCall implements ToolCallCmp.
func (m *toolCallCmp) GetToolCall() message.ToolCall {
	return m.call
}

// GetToolResult implements ToolCallCmp.
func (m *toolCallCmp) GetToolResult() message.ToolResult {
	return m.result
}

func (m *toolCallCmp) renderPending() string {
	return fmt.Sprintf("%s: %s", prettifyToolName(m.call.Name), m.anim.View())
}

func (m *toolCallCmp) style() lipgloss.Style {
	t := theme.CurrentTheme()
	borderStyle := lipgloss.NormalBorder()
	if m.focused {
		borderStyle = lipgloss.DoubleBorder()
	}
	return styles.BaseStyle().
		BorderLeft(true).
		Foreground(t.TextMuted()).
		BorderForeground(t.TextMuted()).
		BorderStyle(borderStyle)
}

func (m *toolCallCmp) textWidth() int {
	return m.width - 2 // take into account the border and PaddingLeft
}

func (m *toolCallCmp) fit(content string, width int) string {
	t := theme.CurrentTheme()
	lineStyle := lipgloss.NewStyle().Background(t.BackgroundSecondary()).Foreground(t.TextMuted())
	dots := lineStyle.Render("...")
	return ansi.Truncate(content, width, dots)
}

func (m *toolCallCmp) Blur() tea.Cmd {
	m.focused = false
	return nil
}

func (m *toolCallCmp) Focus() tea.Cmd {
	m.focused = true
	return nil
}

// IsFocused implements MessageModel.
func (m *toolCallCmp) IsFocused() bool {
	return m.focused
}

func (m *toolCallCmp) GetSize() (int, int) {
	return m.width, 0
}

func (m *toolCallCmp) SetSize(width int, height int) tea.Cmd {
	m.width = width
	return nil
}

func (m *toolCallCmp) shouldSpin() bool {
	if !m.call.Finished {
		return true
	} else if m.result.ToolCallID != m.call.ID {
		return true
	}
	return false
}

func (m *toolCallCmp) Spinning() bool {
	return m.spinning
}
