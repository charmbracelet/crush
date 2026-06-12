package chat

import (
	"encoding/json"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// CallHierarchyToolMessageItem is a message item that represents a call hierarchy tool call.
type CallHierarchyToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*CallHierarchyToolMessageItem)(nil)

// NewCallHierarchyToolMessageItem creates a new [CallHierarchyToolMessageItem].
func NewCallHierarchyToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &CallHierarchyToolRenderContext{}, canceled)
}

// CallHierarchyToolRenderContext renders call hierarchy tool messages.
type CallHierarchyToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (r *CallHierarchyToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Call Hierarchy", opts.Anim, opts.Compact)
	}

	var params tools.CallHierarchyParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	direction := "incoming"
	if params.Direction == "outgoing" {
		direction = "outgoing"
	}
	header := toolHeader(sty, opts.Status, "Call Hierarchy", cappedWidth, opts.Compact, params.Symbol, direction)
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
}
