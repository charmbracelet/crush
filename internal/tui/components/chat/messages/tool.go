package messages

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type ToolCallCmp interface {
	util.Model
	layout.Sizeable
	layout.Focusable
}

type toolCallCmp struct {
	width   int
	focused bool

	call      message.ToolCall
	result    message.ToolResult
	cancelled bool
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

func NewToolCallCmp(tc message.ToolCall, opts ...ToolCallOption) ToolCallCmp {
	m := &toolCallCmp{
		call: tc,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *toolCallCmp) Init() tea.Cmd {
	return nil
}

func (m *toolCallCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (v *toolCallCmp) renderPending() string {
	return fmt.Sprintf("%s: %s", prettifyToolName(v.call.Name), toolAction(v.call.Name))
}

func (msg *toolCallCmp) style() lipgloss.Style {
	t := theme.CurrentTheme()
	borderStyle := lipgloss.NormalBorder()
	if msg.focused {
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

func (m *toolCallCmp) toolName() string {
	switch m.call.Name {
	case agent.AgentToolName:
		return "Task"
	case tools.BashToolName:
		return "Bash"
	case tools.EditToolName:
		return "Edit"
	case tools.FetchToolName:
		return "Fetch"
	case tools.GlobToolName:
		return "Glob"
	case tools.GrepToolName:
		return "Grep"
	case tools.LSToolName:
		return "List"
	case tools.SourcegraphToolName:
		return "Sourcegraph"
	case tools.ViewToolName:
		return "View"
	case tools.WriteToolName:
		return "Write"
	case tools.PatchToolName:
		return "Patch"
	default:
		return m.call.Name
	}
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
