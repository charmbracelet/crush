package chat

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// -----------------------------------------------------------------------------
// Crush Instance Tool
// -----------------------------------------------------------------------------

// CrushInstanceToolMessageItem is a message item that represents a crush instance tool call.
type CrushInstanceToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*CrushInstanceToolMessageItem)(nil)

// NewCrushInstanceToolMessageItem creates a new [CrushInstanceToolMessageItem].
func NewCrushInstanceToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &CrushInstanceToolRenderContext{}, canceled)
}

// CrushInstanceToolRenderContext renders crush instance tool messages.
type CrushInstanceToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (t *CrushInstanceToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Crush Instance", opts.Anim)
	}

	var params tools.CrushInstanceParams
	var meta tools.CrushInstanceMetadata
	var headerText string
	var body string

	// Parse params for display
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err == nil {
		headerText = fmt.Sprintf("Spawning Crush instance")
		if params.Model != "" {
			headerText += fmt.Sprintf(" (model: %s)", params.Model)
		}
	}

	// If we have metadata, use it for richer display.
	if opts.HasResult() && opts.Result.Metadata != "" {
		if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil {
			if meta.IsRunning {
				headerText = fmt.Sprintf("Crush instance running (PID: %s)", meta.ProcessID)
			} else if meta.Completed {
				headerText = fmt.Sprintf("Crush instance completed (PID: %s)", meta.ProcessID)
			} else {
				headerText = fmt.Sprintf("Crush instance failed (PID: %s)", meta.ProcessID)
			}

			if !meta.Completed && meta.IsRunning {
				body = sty.Tool.StateWaiting.Render("Subprocess is running...")
			}
		}
	}

	toolParams := []string{headerText}
	header := toolHeader(sty, opts.Status, "Crush Instance", cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if body == "" {
		return header
	}

	return joinToolParts(header, sty.Tool.Body.Render(body))
}
