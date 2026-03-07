package chat

import (
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// NewSessionToolMessageItem is a message item that represents a new_session tool call.
type NewSessionToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*NewSessionToolMessageItem)(nil)

// NewNewSessionToolMessageItem creates a new [NewSessionToolMessageItem].
func NewNewSessionToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &NewSessionToolRenderContext{}, canceled)
}

// NewSessionToolRenderContext renders new_session tool messages.
type NewSessionToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (r *NewSessionToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "New Session", opts.Anim)
	}

	header := toolHeader(sty, opts.Status, "New Session", cappedWidth, opts.Compact)
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
