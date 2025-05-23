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

// ToolCallCmp defines the interface for tool call components in the chat interface.
// It manages the display of tool execution including pending states, results, and errors.
type ToolCallCmp interface {
	util.Model                         // Basic Bubble Tea model interface
	layout.Sizeable                    // Width/height management
	layout.Focusable                   // Focus state management
	GetToolCall() message.ToolCall     // Access to tool call data
	GetToolResult() message.ToolResult // Access to tool result data
	SetToolResult(message.ToolResult)  // Update tool result
	SetToolCall(message.ToolCall)      // Update tool call
	SetCancelled()                     // Mark as cancelled
	ParentMessageId() string           // Get parent message ID
	Spinning() bool                    // Animation state for pending tools
}

// toolCallCmp implements the ToolCallCmp interface for displaying tool calls.
// It handles rendering of tool execution states including pending, completed, and error states.
type toolCallCmp struct {
	width   int  // Component width for text wrapping
	focused bool // Focus state for border styling

	// Tool call data and state
	parentMessageId string             // ID of the message that initiated this tool call
	call            message.ToolCall   // The tool call being executed
	result          message.ToolResult // The result of the tool execution
	cancelled       bool               // Whether the tool call was cancelled

	// Animation state for pending tool calls
	spinning bool       // Whether to show loading animation
	anim     util.Model // Animation component for pending states
}

// ToolCallOption provides functional options for configuring tool call components
type ToolCallOption func(*toolCallCmp)

// WithToolCallCancelled marks the tool call as cancelled
func WithToolCallCancelled() ToolCallOption {
	return func(m *toolCallCmp) {
		m.cancelled = true
	}
}

// WithToolCallResult sets the initial tool result
func WithToolCallResult(result message.ToolResult) ToolCallOption {
	return func(m *toolCallCmp) {
		m.result = result
	}
}

// NewToolCallCmp creates a new tool call component with the given parent message ID,
// tool call, and optional configuration
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

// Init initializes the tool call component and starts animations if needed.
// Returns a command to start the animation for pending tool calls.
func (m *toolCallCmp) Init() tea.Cmd {
	m.spinning = m.shouldSpin()
	logging.Info("Initializing tool call spinner", "tool_call", m.call.Name, "spinning", m.spinning)
	if m.spinning {
		return m.anim.Init()
	}
	return nil
}

// Update handles incoming messages and updates the component state.
// Manages animation updates for pending tool calls.
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

// View renders the tool call component based on its current state.
// Shows either a pending animation or the tool-specific rendered result.
func (m *toolCallCmp) View() string {
	box := m.style()

	if !m.call.Finished && !m.cancelled {
		return box.PaddingLeft(1).Render(m.renderPending())
	}

	r := registry.lookup(m.call.Name)
	return box.PaddingLeft(1).Render(r.Render(m))
}

// State management methods

// SetCancelled marks the tool call as cancelled
func (m *toolCallCmp) SetCancelled() {
	m.cancelled = true
}

// SetToolCall updates the tool call data and stops spinning if finished
func (m *toolCallCmp) SetToolCall(call message.ToolCall) {
	m.call = call
	if m.call.Finished {
		m.spinning = false
	}
}

// ParentMessageId returns the ID of the message that initiated this tool call
func (m *toolCallCmp) ParentMessageId() string {
	return m.parentMessageId
}

// SetToolResult updates the tool result and stops the spinning animation
func (m *toolCallCmp) SetToolResult(result message.ToolResult) {
	m.result = result
	m.spinning = false
}

// GetToolCall returns the current tool call data
func (m *toolCallCmp) GetToolCall() message.ToolCall {
	return m.call
}

// GetToolResult returns the current tool result data
func (m *toolCallCmp) GetToolResult() message.ToolResult {
	return m.result
}

// Rendering methods

// renderPending displays the tool name with a loading animation for pending tool calls
func (m *toolCallCmp) renderPending() string {
	return fmt.Sprintf("%s: %s", prettifyToolName(m.call.Name), m.anim.View())
}

// style returns the lipgloss style for the tool call component.
// Applies muted colors and focus-dependent border styles.
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

// textWidth calculates the available width for text content,
// accounting for borders and padding
func (m *toolCallCmp) textWidth() int {
	return m.width - 2 // take into account the border and PaddingLeft
}

// fit truncates content to fit within the specified width with ellipsis
func (m *toolCallCmp) fit(content string, width int) string {
	t := theme.CurrentTheme()
	lineStyle := lipgloss.NewStyle().Background(t.BackgroundSecondary()).Foreground(t.TextMuted())
	dots := lineStyle.Render("...")
	return ansi.Truncate(content, width, dots)
}

// Focus management methods

// Blur removes focus from the tool call component
func (m *toolCallCmp) Blur() tea.Cmd {
	m.focused = false
	return nil
}

// Focus sets focus on the tool call component
func (m *toolCallCmp) Focus() tea.Cmd {
	m.focused = true
	return nil
}

// IsFocused returns whether the tool call component is currently focused
func (m *toolCallCmp) IsFocused() bool {
	return m.focused
}

// Size management methods

// GetSize returns the current dimensions of the tool call component
func (m *toolCallCmp) GetSize() (int, int) {
	return m.width, 0
}

// SetSize updates the width of the tool call component for text wrapping
func (m *toolCallCmp) SetSize(width int, height int) tea.Cmd {
	m.width = width
	return nil
}

// shouldSpin determines whether the tool call should show a loading animation.
// Returns true if the tool call is not finished or if the result doesn't match the call ID.
func (m *toolCallCmp) shouldSpin() bool {
	if !m.call.Finished {
		return true
	} else if m.result.ToolCallID != m.call.ID {
		return true
	}
	return false
}

// Spinning returns whether the tool call is currently showing a loading animation
func (m *toolCallCmp) Spinning() bool {
	return m.spinning
}
