package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// -----------------------------------------------------------------------------
// Agent Tool
// -----------------------------------------------------------------------------

// NestedToolContainer is an interface for tool items that can contain nested tool calls.
type NestedToolContainer interface {
	NestedTools() []ToolMessageItem
	SetNestedTools(tools []ToolMessageItem)
	AddNestedTool(tool ToolMessageItem)
}

// AgentToolMessageItem is a message item that represents an agent tool call.
type AgentToolMessageItem struct {
	*baseToolMessageItem

	nestedTools []ToolMessageItem
	// backgroundRunning reports whether the item's background subagent is
	// still running. It only matters for background items.
	backgroundRunning bool
}

var (
	_ ToolMessageItem     = (*AgentToolMessageItem)(nil)
	_ NestedToolContainer = (*AgentToolMessageItem)(nil)
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
		if state.IsCanceled() {
			return false
		}
		return !state.HasResult() || (t.backgroundRunning && backgroundMetadata(state.Result).Background)
	}
	return t
}

// backgroundMetadata decodes the metadata the agent tool attaches to its
// result when the subagent was started in the background.
func backgroundMetadata(result *message.ToolResult) agent.AgentBackgroundMetadata {
	var meta agent.AgentBackgroundMetadata
	if result != nil {
		_ = json.Unmarshal([]byte(result.Metadata), &meta)
	}
	return meta
}

// SetBackgroundRunning sets whether the item's background subagent is still
// running. Items that were not started in the background ignore it.
func (a *AgentToolMessageItem) SetBackgroundRunning(running bool) {
	if a.backgroundRunning == running {
		return
	}
	a.backgroundRunning = running
	a.clearCache()
	a.Bump()
}

// Advance implements [Animatable].
//
// Advances the parent's own spinner and every spinning nested tool in
// one frame, bumping the parent's F6 list-cache version. Nested tools
// are not list entries of their own — their IDs map to this parent's
// index in idInxMap and their renders are embedded inline in this
// parent's output — so the list only checks the parent's version.
// Without the bump, the list cache would serve the previously rendered
// frame indefinitely and the spinner would appear frozen.
func (a *AgentToolMessageItem) Advance() bool {
	if !a.isSpinning() {
		return false
	}
	changed := a.anim.Advance()
	changed = advanceNested(a.nestedTools) || changed
	if changed {
		a.Bump()
	}
	return changed
}

// restyleAnim implements [animRestyler]. It also reaches nested tools,
// which are not list entries of their own and therefore never receive
// the call directly.
func (a *AgentToolMessageItem) restyleAnim() {
	a.baseToolMessageItem.restyleAnim()
	for _, nested := range a.nestedTools {
		if ar, ok := nested.(animRestyler); ok {
			ar.restyleAnim()
		}
	}
}

// advanceNested advances every spinning animatable tool in tools and
// reports whether any of them changed.
func advanceNested(tools []ToolMessageItem) bool {
	changed := false
	for _, nestedTool := range tools {
		if s, ok := nestedTool.(Animatable); ok && s.Spinning() && s.Advance() {
			changed = true
		}
	}
	return changed
}

// NestedTools returns the nested tools.
func (a *AgentToolMessageItem) NestedTools() []ToolMessageItem {
	return a.nestedTools
}

// SetNestedTools sets the nested tools.
//
// SetNestedTools always bumps the version. The previous design
// deduped when the slice's length and element pointers were
// unchanged, but the live update path in internal/ui/model/ui.go
// mutates existing children in place (SetToolCall / SetResult on the
// same pointers) and then calls SetNestedTools with the same slice.
// Pointer-equality dedupe in that case skips the parent Bump even
// though the parent's rendered output (which embeds the children
// inline) has changed, leaving a stale parent entry in the list
// cache. Always bumping is cheap (one uint64 increment) and called
// at most once per agent event; in the rare case the slice is
// truly unchanged the worst case is one extra parent re-render
// while every child cache hit stays warm.
func (a *AgentToolMessageItem) SetNestedTools(tools []ToolMessageItem) {
	a.nestedTools = tools
	a.clearCache()
	a.Bump()
}

// AddNestedTool adds a nested tool.
func (a *AgentToolMessageItem) AddNestedTool(tool ToolMessageItem) {
	// Mark nested tools as simple (compact) rendering.
	if s, ok := tool.(Compactable); ok {
		s.SetCompact(true)
	}
	a.nestedTools = append(a.nestedTools, tool)
	a.clearCache()
	a.Bump()
}

// collapsedNestedTools is how many of an agent's latest tool calls show while
// its item is collapsed. Drawing every call made a busy agent's item slower on
// each frame and taller than the screen.
const collapsedNestedTools = 10

// addNestedTools adds an agent's tool calls to its tree, only the latest ones
// unless the item is expanded.
func addNestedTools(sty *styles.Styles, root *tree.Tree, nested []ToolMessageItem, width int, expanded bool) {
	if hidden := len(nested) - collapsedNestedTools; hidden > 0 && !expanded {
		root.Child(sty.Tool.ParamMain.Render(fmt.Sprintf("… %d earlier tool calls", hidden)))
		nested = nested[hidden:]
	}
	for _, tool := range nested {
		root.Child(tool.Render(width))
	}
}

// AgentToolRenderContext renders agent tool messages.
type AgentToolRenderContext struct {
	agent *AgentToolMessageItem
}

// agentRenderLabel returns the header label for an agent tool row: the plain
// "Agent" string for the default task agent, or "Agent: <name>" when a
// specialized subagent has been dispatched.
func agentRenderLabel(subagentType string) string {
	if subagentType == "" || subagentType == config.AgentTask {
		return "Agent"
	}
	return "Agent: " + subagentType
}

// RenderTool implements the [ToolRenderer] interface.
func (r *AgentToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)

	var params agent.AgentParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	label := agentRenderLabel(params.SubagentType)
	background := backgroundMetadata(opts.Result)
	if background.Background {
		label = "Background: " + background.Name
		if opts.IsSpinning {
			opts.Status = ToolStatusRunning
		}
	}

	if !opts.ToolCall.Finished && !opts.IsCanceled() && len(r.agent.nestedTools) == 0 {
		return pendingTool(sty, label, opts.Anim, opts.Compact)
	}

	prompt := params.Prompt
	if !opts.ExpandedContent {
		prompt = strings.ReplaceAll(prompt, "\n", " ")
	}

	header := toolHeader(sty, opts.Status, label, cappedWidth, opts)
	if opts.Compact {
		return header
	}

	// Build the task tag and prompt.
	taskTag := sty.Tool.AgentTaskTag.Render("Task")
	taskTagWidth := lipgloss.Width(taskTag)

	// Calculate remaining width for prompt.
	remainingWidth := min(cappedWidth-taskTagWidth-3, maxTextWidth-taskTagWidth-3) // -3 for spacing

	promptText := sty.Tool.AgentPrompt.Width(remainingWidth).Render(prompt)

	header = lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			taskTag,
			" ",
			promptText,
		),
	)

	// Build tree with nested tool calls.
	childTools := tree.Root(header)

	addNestedTools(sty, childTools, r.agent.nestedTools, remainingWidth, opts.ExpandedContent)

	// Build parts.
	var parts []string
	parts = append(parts, roundedTree(childTools, taskTagWidth-5).String())

	// Show animation if still running.
	if opts.IsSpinning {
		parts = append(parts, "", opts.Anim.Render())
	}

	result := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// The immediate result of a background start is not the subagent's
	// answer. The report arrives later as its own message.
	if background.Background {
		hint := sty.Tool.ParamMain.Render("Reports back when finished.")
		return joinToolParts(result, hint)
	}

	// Add body content when completed.
	if opts.HasResult() && opts.Result.Content != "" {
		body := toolOutputMarkdownContent(sty, opts.Result.Content, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
		return joinToolParts(result, body)
	}

	return result
}

// -----------------------------------------------------------------------------
// Agentic Fetch Tool
// -----------------------------------------------------------------------------

// AgenticFetchToolMessageItem is a message item that represents an agentic fetch tool call.
type AgenticFetchToolMessageItem struct {
	*baseToolMessageItem

	nestedTools []ToolMessageItem
}

var (
	_ ToolMessageItem     = (*AgenticFetchToolMessageItem)(nil)
	_ NestedToolContainer = (*AgenticFetchToolMessageItem)(nil)
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

// Advance implements [Animatable]. See [AgentToolMessageItem.Advance]
// for the parent-bump rationale; without an override the embedded base
// Advance would never advance the nested children.
func (a *AgenticFetchToolMessageItem) Advance() bool {
	if a.result != nil || a.Status() == ToolStatusCanceled {
		return false
	}
	changed := a.anim.Advance()
	changed = advanceNested(a.nestedTools) || changed
	if changed {
		a.Bump()
	}
	return changed
}

// restyleAnim implements [animRestyler]. See
// [AgentToolMessageItem.restyleAnim] for the nested-tool rationale.
func (a *AgenticFetchToolMessageItem) restyleAnim() {
	a.baseToolMessageItem.restyleAnim()
	for _, nested := range a.nestedTools {
		if ar, ok := nested.(animRestyler); ok {
			ar.restyleAnim()
		}
	}
}

// NestedTools returns the nested tools.
func (a *AgenticFetchToolMessageItem) NestedTools() []ToolMessageItem {
	return a.nestedTools
}

// SetNestedTools sets the nested tools. Always bumps the version;
// see [AgentToolMessageItem.SetNestedTools] for the rationale.
func (a *AgenticFetchToolMessageItem) SetNestedTools(tools []ToolMessageItem) {
	a.nestedTools = tools
	a.clearCache()
	a.Bump()
}

// AddNestedTool adds a nested tool.
func (a *AgenticFetchToolMessageItem) AddNestedTool(tool ToolMessageItem) {
	// Mark nested tools as simple (compact) rendering.
	if s, ok := tool.(Compactable); ok {
		s.SetCompact(true)
	}
	a.nestedTools = append(a.nestedTools, tool)
	a.clearCache()
	a.Bump()
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
	if !opts.ToolCall.Finished && !opts.IsCanceled() && len(r.fetch.nestedTools) == 0 {
		return pendingTool(sty, "Agentic Fetch", opts.Anim, opts.Compact)
	}

	var params agenticFetchParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	prompt := params.Prompt
	if !opts.ExpandedContent {
		prompt = strings.ReplaceAll(prompt, "\n", " ")
	}

	// Build header with optional URL param.
	var toolParams []string
	if params.URL != "" {
		toolParams = append(toolParams, params.URL)
	}

	header := toolHeader(sty, opts.Status, "Agentic Fetch", cappedWidth, opts, toolParams...)
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

	// Build tree with nested tool calls.
	childTools := tree.Root(header)

	addNestedTools(sty, childTools, r.fetch.nestedTools, remainingWidth, opts.ExpandedContent)

	// Build parts.
	var parts []string
	parts = append(parts, roundedTree(childTools, promptTagWidth-5).String())

	// Show animation if still running.
	if !opts.HasResult() && !opts.IsCanceled() {
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

// -----------------------------------------------------------------------------
// Agent Send and Stop Tools
// -----------------------------------------------------------------------------

// NewAgentSendToolMessageItem creates a message item for an agent_send call.
func NewAgentSendToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, backgroundAgentRenderer("Agent: Send", true), canceled)
}

// NewAgentStopToolMessageItem creates a message item for an agent_stop call.
func NewAgentStopToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, backgroundAgentRenderer("Agent: Stop", false), canceled)
}

// backgroundAgentRenderer renders agent_send and agent_stop calls: the
// subagent name (and the message, when withMessage is set) in the header and
// the result text as the body.
func backgroundAgentRenderer(label string, withMessage bool) ToolRenderer {
	return ToolRendererFunc(func(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
		cappedWidth := cappedMessageWidth(width)
		if opts.IsPending() {
			return pendingTool(sty, label, opts.Anim, opts.Compact)
		}

		var params agent.AgentSendParams
		_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

		toolParams := []string{params.Name}
		if withMessage && params.Message != "" {
			toolParams = append(toolParams, strings.ReplaceAll(params.Message, "\n", " "))
		}

		header := toolHeader(sty, opts.Status, label, cappedWidth, opts, toolParams...)
		if opts.Compact {
			return header
		}

		if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
			return joinToolParts(header, earlyState)
		}

		if opts.HasEmptyResult() {
			return header
		}

		bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
		body := sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
		return joinToolParts(header, body)
	})
}
