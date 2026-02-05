package chat

import (
	"encoding/json"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// -----------------------------------------------------------------------------
// Diagnostics Tool
// -----------------------------------------------------------------------------

// DiagnosticsToolMessageItem is a message item that represents a diagnostics tool call.
type DiagnosticsToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*DiagnosticsToolMessageItem)(nil)

// NewDiagnosticsToolMessageItem creates a new [DiagnosticsToolMessageItem].
func NewDiagnosticsToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &DiagnosticsToolRenderContext{}, canceled)
}

// DiagnosticsToolRenderContext renders diagnostics tool messages.
type DiagnosticsToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (d *DiagnosticsToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Diagnostics", opts.Anim)
	}

	var params tools.DiagnosticsParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	// Show "project" if no file path, otherwise show the file path.
	mainParam := "project"
	if params.FilePath != "" {
		mainParam = fsext.PrettyPath(params.FilePath)
	}

	header := toolHeader(sty, opts.Status, "Diagnostics", cappedWidth, opts.Compact, mainParam)
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
