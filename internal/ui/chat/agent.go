package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// maxAgentPromptDisplayLines is the maximum number of lines to show for a
// subagent prompt or description in the main session view before truncating.
const maxAgentPromptDisplayLines = 3

// maxCollapsedAgentNestedTools is the number of nested tool calls rendered in
// collapsed mode before the user expands the agent block.
const maxCollapsedAgentNestedTools = 10

// -----------------------------------------------------------------------------
// Agent Tool
// -----------------------------------------------------------------------------

// NestedToolContainer is an interface for tool items that can contain nested tool calls.
type NestedToolContainer interface {
	NestedTools() []ToolMessageItem
	SetNestedTools(tools []ToolMessageItem)
	AddNestedTool(tool ToolMessageItem)
}

// ChildSessionStatusSetter updates the transient child-session status shown on
// parent agent tool items while the delegated work is still running.
type ChildSessionStatusSetter interface {
	SetChildSessionStatus(text string, isError bool)
	ClearChildSessionStatus()
}

// AgentToolMessageItem is a message item that represents an agent tool call.
type AgentToolMessageItem struct {
	*baseToolMessageItem

	nestedTools    []ToolMessageItem
	nestedExpanded bool

	childStatusText    string
	childStatusIsError bool
}

var (
	_ ToolMessageItem          = (*AgentToolMessageItem)(nil)
	_ NestedToolContainer      = (*AgentToolMessageItem)(nil)
	_ ChildSessionStatusSetter = (*AgentToolMessageItem)(nil)
)

// NewAgentToolMessageItem creates a new [AgentToolMessageItem].
func NewAgentToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) *AgentToolMessageItem {
	t := &AgentToolMessageItem{}
	t.baseToolMessageItem = newBaseToolMessageItem(sty, toolCall, result, &AgentToolRenderContext{agent: t}, canceled)
	// For the agent tool we keep spinning until the tool call is finished.
	t.spinningFunc = func(state SpinningState) bool {
		return !state.HasResult() && !state.IsCanceled()
	}
	return t
}

// Animate progresses the message animation if it should be spinning.
func (a *AgentToolMessageItem) Animate(msg anim.StepMsg) tea.Cmd {
	if a.result != nil || a.Status() == ToolStatusCanceled {
		return nil
	}
	if msg.ID == a.ID() {
		return a.anim.Animate(msg)
	}
	for _, nestedTool := range a.nestedTools {
		if msg.ID != nestedTool.ID() {
			continue
		}
		if s, ok := nestedTool.(Animatable); ok {
			return s.Animate(msg)
		}
	}
	return nil
}

// NestedTools returns the nested tools.
func (a *AgentToolMessageItem) NestedTools() []ToolMessageItem {
	return a.nestedTools
}

// SetNestedTools sets the nested tools.
func (a *AgentToolMessageItem) SetNestedTools(tools []ToolMessageItem) {
	a.nestedTools = tools
	a.clearCache()
}

// AddNestedTool adds a nested tool.
func (a *AgentToolMessageItem) AddNestedTool(tool ToolMessageItem) {
	// Mark nested tools as simple (compact) rendering.
	if s, ok := tool.(Compactable); ok {
		s.SetCompact(true)
	}
	a.nestedTools = append(a.nestedTools, tool)
	a.clearCache()
}

// ToggleExpanded toggles the nested tool list expansion state.
func (a *AgentToolMessageItem) ToggleExpanded() bool {
	a.nestedExpanded = !a.nestedExpanded
	a.expandedContent = a.nestedExpanded
	a.clearCache()
	return a.nestedExpanded
}

// SetChildSessionStatus stores transient child-session status text.
func (a *AgentToolMessageItem) SetChildSessionStatus(text string, isError bool) {
	if a.childStatusText == text && a.childStatusIsError == isError {
		return
	}
	a.childStatusText = text
	a.childStatusIsError = isError
	a.clearCache()
}

// ClearChildSessionStatus removes transient child-session status text.
func (a *AgentToolMessageItem) ClearChildSessionStatus() {
	if a.childStatusText == "" && !a.childStatusIsError {
		return
	}
	a.childStatusText = ""
	a.childStatusIsError = false
	a.clearCache()
}

// AgentToolRenderContext renders agent tool messages.
type AgentToolRenderContext struct {
	agent *AgentToolMessageItem
}

// RenderTool implements the [ToolRenderer] interface.
func (r *AgentToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() && len(r.agent.nestedTools) == 0 && r.agent.childStatusText == "" {
		return pendingTool(sty, "Agent", opts.Anim, opts.Compact)
	}

	var params agent.AgentParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	prompt := strings.ReplaceAll(params.Prompt, "\n", " ")
	description := strings.TrimSpace(params.Description)
	if description == "" {
		description = prompt
	}
	description = strings.ReplaceAll(description, "\n", " ")
	subagentType := titleCase(config.CanonicalSubagentID(params.SubagentType))

	header := toolHeader(sty, opts.Status, "Agent", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	// Build the subagent tag and description.
	taskTag := sty.Tool.AgentTaskTag.Render(subagentType)
	taskTagWidth := lipgloss.Width(taskTag)

	// Calculate remaining width for the title.
	remainingWidth := min(cappedWidth-taskTagWidth-3, maxTextWidth-taskTagWidth-3) // -3 for spacing

	descriptionText := sty.Tool.AgentPrompt.Width(remainingWidth).Render(truncateAgentPrompt(description, remainingWidth))
	headerParts := []string{
		header,
		"",
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			taskTag,
			" ",
			descriptionText,
		),
	}
	if prompt != "" && prompt != description {
		promptTag := sty.Tool.AgenticFetchPromptTag.Render("Prompt")
		promptWidth := min(cappedWidth-lipgloss.Width(promptTag)-3, maxTextWidth-lipgloss.Width(promptTag)-3)
		promptText := sty.Tool.AgentPrompt.Width(promptWidth).Render(truncateAgentPrompt(prompt, promptWidth))
		headerParts = append(headerParts, lipgloss.JoinHorizontal(lipgloss.Left, promptTag, " ", promptText))
	}

	header = lipgloss.JoinVertical(lipgloss.Left, headerParts...)

	visibleNestedTools, hiddenNestedTools := agentNestedToolWindow(r.agent.nestedTools, r.agent.nestedExpanded)
	header = renderAgentHeaderWithToggle(sty, header, remainingWidth, r.agent.nestedExpanded, len(r.agent.nestedTools), hiddenNestedTools)

	// Build tree with nested tool calls.
	childTools := tree.Root(header)

	for _, nestedTool := range visibleNestedTools {
		childView := nestedTool.Render(remainingWidth)
		childTools.Child(childView)
	}

	// Build parts.
	var parts []string
	parts = append(parts, childTools.Enumerator(roundedEnumerator(2, taskTagWidth-5)).String())

	if !opts.HasResult() {
		if status := renderChildSessionStatus(sty, remainingWidth, r.agent.childStatusText, r.agent.childStatusIsError); status != "" {
			parts = append(parts, "", status)
		}
	}

	// Show animation if still running.
	if opts.IsSpinning && !opts.HasResult() && !opts.IsCanceled() {
		parts = append(parts, "", opts.Anim.Render())
	}

	result := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Add body content when completed.
	if opts.HasResult() && opts.Result.Content != "" {
		body := toolOutputMarkdownContent(sty, opts.Result.Content, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
		return joinToolParts(result, body)
	}

	return result
}

// truncateAgentPrompt truncates a single-line string to fit within
// maxAgentPromptDisplayLines lines at the given column width. If the string is
// truncated, "…" is appended to the last visible character.
func truncateAgentPrompt(text string, lineWidth int) string {
	if lineWidth <= 0 {
		return text
	}
	maxRunes := lineWidth * maxAgentPromptDisplayLines
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes-1]) + "…"
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// -----------------------------------------------------------------------------
// Agentic Fetch Tool
// -----------------------------------------------------------------------------

// AgenticFetchToolMessageItem is a message item that represents an agentic fetch tool call.
type AgenticFetchToolMessageItem struct {
	*baseToolMessageItem

	nestedTools    []ToolMessageItem
	nestedExpanded bool

	childStatusText    string
	childStatusIsError bool
}

var (
	_ ToolMessageItem          = (*AgenticFetchToolMessageItem)(nil)
	_ NestedToolContainer      = (*AgenticFetchToolMessageItem)(nil)
	_ ChildSessionStatusSetter = (*AgenticFetchToolMessageItem)(nil)
)

// NewAgenticFetchToolMessageItem creates a new [AgenticFetchToolMessageItem].
func NewAgenticFetchToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) *AgenticFetchToolMessageItem {
	t := &AgenticFetchToolMessageItem{}
	t.baseToolMessageItem = newBaseToolMessageItem(sty, toolCall, result, &AgenticFetchToolRenderContext{fetch: t}, canceled)
	// For the agentic fetch tool we keep spinning until the tool call is finished.
	t.spinningFunc = func(state SpinningState) bool {
		return !state.HasResult() && !state.IsCanceled()
	}
	return t
}

// NestedTools returns the nested tools.
func (a *AgenticFetchToolMessageItem) NestedTools() []ToolMessageItem {
	return a.nestedTools
}

// SetNestedTools sets the nested tools.
func (a *AgenticFetchToolMessageItem) SetNestedTools(tools []ToolMessageItem) {
	a.nestedTools = tools
	a.clearCache()
}

// AddNestedTool adds a nested tool.
func (a *AgenticFetchToolMessageItem) AddNestedTool(tool ToolMessageItem) {
	// Mark nested tools as simple (compact) rendering.
	if s, ok := tool.(Compactable); ok {
		s.SetCompact(true)
	}
	a.nestedTools = append(a.nestedTools, tool)
	a.clearCache()
}

// ToggleExpanded toggles the nested tool list expansion state.
func (a *AgenticFetchToolMessageItem) ToggleExpanded() bool {
	a.nestedExpanded = !a.nestedExpanded
	a.expandedContent = a.nestedExpanded
	a.clearCache()
	return a.nestedExpanded
}

// SetChildSessionStatus stores transient child-session status text.
func (a *AgenticFetchToolMessageItem) SetChildSessionStatus(text string, isError bool) {
	if a.childStatusText == text && a.childStatusIsError == isError {
		return
	}
	a.childStatusText = text
	a.childStatusIsError = isError
	a.clearCache()
}

// ClearChildSessionStatus removes transient child-session status text.
func (a *AgenticFetchToolMessageItem) ClearChildSessionStatus() {
	if a.childStatusText == "" && !a.childStatusIsError {
		return
	}
	a.childStatusText = ""
	a.childStatusIsError = false
	a.clearCache()
}

// AgenticFetchToolRenderContext renders agentic fetch tool messages.
type AgenticFetchToolRenderContext struct {
	fetch *AgenticFetchToolMessageItem
}

// agenticFetchParams matches tools.AgenticFetchParams.
type agenticFetchParams struct {
	URL    string `json:"url,omitempty"`
	Prompt string `json:"prompt"`
}

// RenderTool implements the [ToolRenderer] interface.
func (r *AgenticFetchToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() && len(r.fetch.nestedTools) == 0 && r.fetch.childStatusText == "" {
		return pendingTool(sty, "Agentic Fetch", opts.Anim, opts.Compact)
	}

	var params agenticFetchParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	prompt := params.Prompt
	prompt = strings.ReplaceAll(prompt, "\n", " ")

	// Build header with optional URL param.
	var toolParams []string
	if params.URL != "" {
		toolParams = append(toolParams, params.URL)
	}

	header := toolHeader(sty, opts.Status, "Agentic Fetch", cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	// Build the prompt tag.
	promptTag := sty.Tool.AgenticFetchPromptTag.Render("Prompt")
	promptTagWidth := lipgloss.Width(promptTag)

	// Calculate remaining width for prompt text.
	remainingWidth := min(cappedWidth-promptTagWidth-3, maxTextWidth-promptTagWidth-3) // -3 for spacing

	promptText := sty.Tool.AgentPrompt.Width(remainingWidth).Render(prompt)

	header = lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			promptTag,
			" ",
			promptText,
		),
	)

	visibleNestedTools, hiddenNestedTools := agentNestedToolWindow(r.fetch.nestedTools, r.fetch.nestedExpanded)
	header = renderAgentHeaderWithToggle(sty, header, remainingWidth, r.fetch.nestedExpanded, len(r.fetch.nestedTools), hiddenNestedTools)

	// Build tree with nested tool calls.
	childTools := tree.Root(header)

	for _, nestedTool := range visibleNestedTools {
		childView := nestedTool.Render(remainingWidth)
		childTools.Child(childView)
	}

	// Build parts.
	var parts []string
	parts = append(parts, childTools.Enumerator(roundedEnumerator(2, promptTagWidth-5)).String())

	if !opts.HasResult() {
		if status := renderChildSessionStatus(sty, remainingWidth, r.fetch.childStatusText, r.fetch.childStatusIsError); status != "" {
			parts = append(parts, "", status)
		}
	}

	// Show animation if still running.
	if opts.IsSpinning && !opts.HasResult() && !opts.IsCanceled() {
		parts = append(parts, "", opts.Anim.Render())
	}

	result := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Add body content when completed.
	if opts.HasResult() && opts.Result.Content != "" {
		body := toolOutputMarkdownContent(sty, opts.Result.Content, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
		return joinToolParts(result, body)
	}

	return result
}

func renderChildSessionStatus(sty *styles.Styles, width int, text string, isError bool) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if text == "" || width <= 0 {
		return ""
	}

	statusTag := sty.Tool.AgenticFetchPromptTag.Render("Status")
	availableWidth := max(0, width-lipgloss.Width(statusTag)-3)
	if availableWidth == 0 {
		return statusTag
	}

	if isError {
		errTag := sty.Tool.ErrorTag.Render("ERROR")
		errText := sty.Tool.ErrorMessage.Render(
			ansi.Truncate(text, max(0, availableWidth-lipgloss.Width(errTag)-1), "…"),
		)
		return lipgloss.JoinHorizontal(
			lipgloss.Left,
			statusTag,
			" ",
			fmt.Sprintf("%s %s", errTag, errText),
		)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusTag,
		" ",
		sty.Tool.StateWaiting.Render(ansi.Truncate(text, availableWidth, "…")),
	)
}

func agentNestedToolWindow(nestedTools []ToolMessageItem, expanded bool) ([]ToolMessageItem, int) {
	if expanded || len(nestedTools) <= maxCollapsedAgentNestedTools {
		return nestedTools, 0
	}

	visible := maxCollapsedAgentNestedTools
	return nestedTools[:visible], len(nestedTools) - visible
}

func renderAgentHeaderWithToggle(sty *styles.Styles, header string, width int, expanded bool, totalNested, hiddenNested int) string {
	if totalNested <= maxCollapsedAgentNestedTools {
		return header
	}

	var toggleLabel string
	if expanded {
		toggleLabel = "▾ Collapse"
	} else {
		toggleLabel = fmt.Sprintf("▸ Expand (%d more)", hiddenNested)
	}

	toggleTag := sty.Tool.AgenticFetchPromptTag.Render(toggleLabel)
	lines := strings.Split(header, "\n")
	if len(lines) == 0 {
		return header
	}

	firstLineWidth := ansi.StringWidth(lines[0])
	if width <= 0 {
		lines[0] = lipgloss.JoinHorizontal(lipgloss.Left, lines[0], " ", toggleTag)
		return strings.Join(lines, "\n")
	}

	availableWidth := max(0, width-firstLineWidth-1)
	if availableWidth == 0 {
		toggleTag = ansi.Truncate(toggleTag, width, "…")
		return lipgloss.JoinVertical(lipgloss.Left, header, toggleTag)
	}

	toggleTag = ansi.Truncate(toggleTag, availableWidth, "…")
	lines[0] = lipgloss.JoinHorizontal(lipgloss.Left, lines[0], " ", toggleTag)
	return strings.Join(lines, "\n")
}
