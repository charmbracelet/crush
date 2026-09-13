package chat

import (
	"encoding/json"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/crush/internal/agent"
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

	// dispatchLabel is set for agent_dispatch calls, which return as soon
	// as the sub-agent starts. Such an item keeps spinning until the
	// sub-agent's report arrives and clears it, since the tool result
	// landing says only that the sub-agent launched, not that it is done.
	dispatchLabel    string
	dispatchReported bool
	dispatchStarted  time.Time
	// dispatchReportedAt is when the report landed. The sidebar retires a
	// sub-agent that has been finished a while, so the section does not
	// grow for the whole session.
	dispatchReportedAt time.Time
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
	t.parseDispatchLabel(toolCall)
	// For the agent tool we keep spinning until the tool call is finished.
	// A dispatched sub-agent runs on past its tool result, so it spins
	// until its report arrives instead.
	t.spinningFunc = func(state SpinningState) bool {
		if t.dispatchLabel != "" {
			return !t.dispatchReported && !state.IsCanceled()
		}
		return !state.HasResult() && !state.IsCanceled()
	}
	return t
}

// parseDispatchLabel reads the sub-agent's label out of an agent_dispatch
// call. The tool call arrives with its input still streaming, so the label is
// re-read on every update until it parses rather than once at construction.
func (a *AgentToolMessageItem) parseDispatchLabel(toolCall message.ToolCall) {
	if toolCall.Name != agent.AgentDispatchToolName || a.dispatchLabel != "" {
		return
	}
	var params agent.AgentDispatchParams
	if json.Unmarshal([]byte(toolCall.Input), &params) == nil && params.Label != "" {
		a.dispatchLabel = params.Label
		a.dispatchStarted = time.Now()
	}
}

// SetToolCall updates the tool call, picking up the dispatch label once the
// streamed input is complete enough to parse.
func (a *AgentToolMessageItem) SetToolCall(tc message.ToolCall) {
	a.baseToolMessageItem.SetToolCall(tc)
	a.parseDispatchLabel(tc)
}

// DispatchLabel returns the label of the sub-agent this item dispatched, or
// the empty string for a synchronous agent call.
func (a *AgentToolMessageItem) DispatchLabel() string { return a.dispatchLabel }

// MarkDispatchRunning puts the item back into its working state after its
// sub-agent was reopened with a follow-up.
func (a *AgentToolMessageItem) MarkDispatchRunning() {
	if !a.dispatchReported {
		return
	}
	a.dispatchReported = false
	a.dispatchStarted = time.Now()
	a.dispatchReportedAt = time.Time{}
	a.clearCache()
	a.Bump()
}

// MarkDispatchReported records that the dispatched sub-agent reported back,
// which stops the item's spinner.
func (a *AgentToolMessageItem) MarkDispatchReported() {
	if a.dispatchReported {
		return
	}
	a.dispatchReported = true
	a.dispatchReportedAt = time.Now()
	a.clearCache()
	a.Bump()
}

// Unresolved implements [ToolMessageItem].
//
// A dispatch call is answered the moment its sub-agent starts, so the tool
// result landing says only that the sub-agent was launched. What settles the
// item is the sub-agent's report, and until that arrives the work is still
// outstanding however complete the tool result looks.
func (a *AgentToolMessageItem) Unresolved() bool {
	if a.dispatchLabel != "" {
		return !a.dispatchReported && a.Status() != ToolStatusCanceled
	}
	return a.baseToolMessageItem.Unresolved()
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
	if a.Status() == ToolStatusCanceled {
		return false
	}
	// A dispatched sub-agent keeps working after its tool result lands, so
	// its animation runs until the report arrives.
	if a.dispatchLabel == "" && a.result != nil {
		return false
	}
	if a.dispatchLabel != "" && a.dispatchReported {
		return false
	}
	changed := a.anim.Advance()
	changed = advanceNested(a.nestedTools) || changed
	if changed {
		a.Bump()
	}
	return changed
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

// DispatchStatus reports what a dispatched sub-agent is up to. The second
// return is false for a synchronous agent call.
func (a *AgentToolMessageItem) DispatchStatus() (DispatchStatus, bool) {
	if a.dispatchLabel == "" {
		return DispatchStatus{}, false
	}
	status := DispatchStatus{
		Label:    a.dispatchLabel,
		Running:  !a.dispatchReported && a.Status() != ToolStatusCanceled,
		Started:  a.dispatchStarted,
		Finished: a.dispatchReportedAt,
	}
	if len(a.nestedTools) > 0 {
		status.Activity = prettifyToolName(a.nestedTools[len(a.nestedTools)-1].ToolCall().Name)
	}
	return status, true
}

// DispatchStatus describes what a dispatched sub-agent is up to.
type DispatchStatus struct {
	Label    string
	Running  bool
	Activity string // Name of the last tool it called.
	Started  time.Time
	Finished time.Time // When its report landed; zero while running.
}

// AgentToolRenderContext renders agent tool messages.
type AgentToolRenderContext struct {
	agent *AgentToolMessageItem
}

// RenderTool implements the [ToolRenderer] interface.
func (r *AgentToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if !opts.ToolCall.Finished && !opts.IsCanceled() && len(r.agent.nestedTools) == 0 {
		return pendingTool(sty, "Agent", opts.Anim, opts.Compact)
	}

	var params agent.AgentParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	prompt := params.Prompt
	if !opts.ExpandedContent {
		prompt = strings.ReplaceAll(prompt, "\n", " ")
	}

	// A dispatch is named for the sub-agent it launched, so its header
	// pairs with the report item that arrives later under the same label.
	name := "Agent"
	var headerParams []string
	if r.agent.dispatchLabel != "" {
		name = "Sub-agent"
		headerParams = []string{r.agent.dispatchLabel}
	}
	header := toolHeader(sty, opts.Status, name, cappedWidth, opts, headerParams...)
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

	for _, nestedTool := range r.agent.nestedTools {
		childView := nestedTool.Render(remainingWidth)
		childTools.Child(childView)
	}

	// Build parts.
	var parts []string
	parts = append(parts, childTools.Enumerator(roundedEnumerator(2, taskTagWidth-5)).String())

	// Show animation if still running. A dispatch has its result the
	// moment the sub-agent starts, so it animates until the report lands
	// instead.
	stillRunning := !opts.HasResult()
	if r.agent.dispatchLabel != "" {
		stillRunning = !r.agent.dispatchReported
	}
	if stillRunning && !opts.IsCanceled() {
		parts = append(parts, "", opts.Anim.Render())
	}

	result := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Add body content when completed. A dispatch result only says the
	// sub-agent started, which is bookkeeping rather than output; its real
	// output arrives later as its own report item.
	if r.agent.dispatchLabel != "" {
		return result
	}
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

	for _, nestedTool := range r.fetch.nestedTools {
		childView := nestedTool.Render(remainingWidth)
		childTools.Child(childView)
	}

	// Build parts.
	var parts []string
	parts = append(parts, childTools.Enumerator(roundedEnumerator(2, promptTagWidth-5)).String())

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
