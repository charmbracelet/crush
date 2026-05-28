package chat

import (
	"encoding/json"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// ReplaceSymbolToolMessageItem is a message item that represents a replace symbol tool call.
type ReplaceSymbolToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*ReplaceSymbolToolMessageItem)(nil)

// NewReplaceSymbolToolMessageItem creates a new [ReplaceSymbolToolMessageItem].
func NewReplaceSymbolToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &ReplaceSymbolToolRenderContext{}, canceled)
}

// ReplaceSymbolToolRenderContext renders replace symbol tool messages.
type ReplaceSymbolToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (r *ReplaceSymbolToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Replace Symbol", opts.Anim, opts.Compact)
	}

	var params tools.ReplaceSymbolParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	header := toolHeader(sty, opts.Status, "Replace Symbol", cappedWidth, opts, params.Symbol, params.FilePath)
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
