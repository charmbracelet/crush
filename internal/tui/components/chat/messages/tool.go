package messages

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	ParentMessageID() string           // Get parent message ID
	Spinning() bool                    // Animation state for pending tools
	GetNestedToolCalls() []ToolCallCmp // Get nested tool calls
	SetNestedToolCalls([]ToolCallCmp)  // Set nested tool calls
	SetIsNested(bool)                  // Set whether this tool call is nested
	SetPermissionRequested()           // Mark permission request
	SetPermissionGranted()             // Mark permission granted
}

// toolCallCmp implements the ToolCallCmp interface for displaying tool calls.
// It handles rendering of tool execution states including pending, completed, and error states.
type toolCallCmp struct {
	width    int  // Component width for text wrapping
	focused  bool // Focus state for border styling
	isNested bool // Whether this tool call is nested within another

	// Tool call data and state
	parentMessageID     string             // ID of the message that initiated this tool call
	call                message.ToolCall   // The tool call being executed
	result              message.ToolResult // The result of the tool execution
	cancelled           bool               // Whether the tool call was cancelled
	permissionRequested bool
	permissionGranted   bool

	// Animation state for pending tool calls
	spinning bool       // Whether to show loading animation
	anim     util.Model // Animation component for pending states

	nestedToolCalls []ToolCallCmp // Nested tool calls for hierarchical display
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

func WithToolCallNested(isNested bool) ToolCallOption {
	return func(m *toolCallCmp) {
		m.isNested = isNested
	}
}

func WithToolCallNestedCalls(calls []ToolCallCmp) ToolCallOption {
	return func(m *toolCallCmp) {
		m.nestedToolCalls = calls
	}
}

func WithToolPermissionRequested() ToolCallOption {
	return func(m *toolCallCmp) {
		m.permissionRequested = true
	}
}

func WithToolPermissionGranted() ToolCallOption {
	return func(m *toolCallCmp) {
		m.permissionGranted = true
	}
}

// NewToolCallCmp creates a new tool call component with the given parent message ID,
// tool call, and optional configuration
func NewToolCallCmp(parentMessageID string, tc message.ToolCall, permissions permission.Service, opts ...ToolCallOption) ToolCallCmp {
	m := &toolCallCmp{
		call:            tc,
		parentMessageID: parentMessageID,
	}
	for _, opt := range opts {
		opt(m)
	}
	t := styles.CurrentTheme()
	m.anim = anim.New(anim.Settings{
		Size:        15,
		Label:       "Working",
		GradColorA:  t.Primary,
		GradColorB:  t.Secondary,
		LabelColor:  t.FgBase,
		CycleColors: true,
	})
	if m.isNested {
		m.anim = anim.New(anim.Settings{
			Size:        10,
			GradColorA:  t.Primary,
			GradColorB:  t.Secondary,
			CycleColors: true,
		})
	}
	return m
}

// Init initializes the tool call component and starts animations if needed.
// Returns a command to start the animation for pending tool calls.
func (m *toolCallCmp) Init() tea.Cmd {
	m.spinning = m.shouldSpin()
	return m.anim.Init()
}

// Update handles incoming messages and updates the component state.
// Manages animation updates for pending tool calls.
func (m *toolCallCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case anim.StepMsg:
		var cmds []tea.Cmd
		for i, nested := range m.nestedToolCalls {
			if nested.Spinning() {
				u, cmd := nested.Update(msg)
				m.nestedToolCalls[i] = u.(ToolCallCmp)
				cmds = append(cmds, cmd)
			}
		}
		if m.spinning {
			u, cmd := m.anim.Update(msg)
			m.anim = u.(util.Model)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// View renders the tool call component based on its current state.
// Shows either a pending animation or the tool-specific rendered result.
func (m *toolCallCmp) View() string {
	box := m.style()

	if !m.call.Finished && !m.cancelled {
		return box.Render(m.renderPending())
	}

	r := registry.lookup(m.call.Name)

	if m.isNested {
		return box.Render(r.Render(m))
	}
	return box.Render(r.Render(m))
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

// ParentMessageID returns the ID of the message that initiated this tool call
func (m *toolCallCmp) ParentMessageID() string {
	return m.parentMessageID
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

// GetNestedToolCalls returns the nested tool calls
func (m *toolCallCmp) GetNestedToolCalls() []ToolCallCmp {
	return m.nestedToolCalls
}

// SetNestedToolCalls sets the nested tool calls
func (m *toolCallCmp) SetNestedToolCalls(calls []ToolCallCmp) {
	m.nestedToolCalls = calls
	for _, nested := range m.nestedToolCalls {
		nested.SetSize(m.width, 0)
	}
}

// SetIsNested sets whether this tool call is nested within another
func (m *toolCallCmp) SetIsNested(isNested bool) {
	m.isNested = isNested
}

// Rendering methods

// renderPending displays the tool name with a loading animation for pending tool calls
func (m *toolCallCmp) renderPending() string {
	t := styles.CurrentTheme()
	icon := t.S().Base.Foreground(t.GreenDark).Render(styles.ToolPending)
	if m.isNested {
		tool := t.S().Base.Foreground(t.FgHalfMuted).Render(prettifyToolName(m.call.Name))
		return fmt.Sprintf("%s %s %s", icon, tool, m.anim.View())
	}
	tool := t.S().Base.Foreground(t.Blue).Render(prettifyToolName(m.call.Name))
	return fmt.Sprintf("%s %s %s", icon, tool, m.anim.View())
}

// style returns the lipgloss style for the tool call component.
// Applies muted colors and focus-dependent border styles.
func (m *toolCallCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()

	if m.isNested {
		return t.S().Muted
	}
	style := t.S().Muted.PaddingLeft(4)

	if m.focused {
		style = style.PaddingLeft(3).BorderStyle(focusedMessageBorder).BorderLeft(true).BorderForeground(t.GreenDark)
	}
	return style
}

// textWidth calculates the available width for text content,
// accounting for borders and padding
func (m *toolCallCmp) textWidth() int {
	if m.isNested {
		return m.width - 6
	}
	return m.width - 5 // take into account the border and PaddingLeft
}

// fit truncates content to fit within the specified width with ellipsis
func (m *toolCallCmp) fit(content string, width int) string {
	t := styles.CurrentTheme()
	lineStyle := t.S().Muted
	dots := lineStyle.Render("…")
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
	for _, nested := range m.nestedToolCalls {
		nested.SetSize(width, height)
	}
	return nil
}

// shouldSpin determines whether the tool call should show a loading animation.
// Returns true if the tool call is not finished or if the result doesn't match the call ID.
func (m *toolCallCmp) shouldSpin() bool {
	return !m.call.Finished
}

// Spinning returns whether the tool call is currently showing a loading animation
func (m *toolCallCmp) Spinning() bool {
	if m.spinning {
		return true
	}
	for _, nested := range m.nestedToolCalls {
		if nested.Spinning() {
			return true
		}
	}
	return m.spinning
}

// SetPermissionRequested marks that a permission request was made for this tool call
func (m *toolCallCmp) SetPermissionRequested() {
	m.permissionRequested = true
}

// SetPermissionGranted marks that permission was granted for this tool call
func (m *toolCallCmp) SetPermissionGranted() {
	m.permissionGranted = true
}
