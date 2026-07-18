package chat

import (
	"encoding/json"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// SymbolsToolMessageItem is a message item that represents a symbols tool call.
type SymbolsToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*SymbolsToolMessageItem)(nil)

// NewSymbolsToolMessageItem creates a new [SymbolsToolMessageItem].
func NewSymbolsToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &SymbolsToolRenderContext{}, canceled)
}

// SymbolsToolRenderContext renders symbols tool messages.
type SymbolsToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (r *SymbolsToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "List Symbols", opts.Anim, opts.Compact)
	}

	var params tools.SymbolsParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	header := toolHeader(sty, opts.Status, "List Symbols", cappedWidth, opts, params.FilePath)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.HasEmptyResult() {
		return header
	}

	// Render as code to preserve tree indentation.
	body := toolOutputCodeContent(sty, params.FilePath, opts.Result.Content, 0, cappedWidth, opts.ExpandedContent)
	return joinToolParts(header, body)
}
