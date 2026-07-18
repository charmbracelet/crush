package chat

import (
	"encoding/json"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/fsext"
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
	// Replace symbol uses full width for diffs, like edit.
	if opts.IsPending() {
		return pendingTool(sty, "Replace Symbol", opts.Anim, opts.Compact)
	}

	var params tools.ReplaceSymbolParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	file := fsext.PrettyPath(params.FilePath)
	header := toolHeader(sty, opts.Status, "Replace Symbol", width, opts, params.Symbol, file)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, width); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() {
		return header
	}

	// Try to render as a diff using metadata.
	var meta tools.ReplaceSymbolResponseMetadata
	if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil && meta.OldContent != "" || meta.NewContent != "" {
		diff := toolOutputDiffContent(sty, file, meta.OldContent, meta.NewContent, width, opts.ExpandedContent)

		// On error, show error above the diff.
		if opts.Result.IsError {
			errLine := toolErrorContent(sty, opts.Result, width)
			return joinToolParts(header, errLine+"\n"+diff)
		}

		return joinToolParts(header, diff)
	}

	// Fallback to plain text if no metadata.
	bodyWidth := width - toolBodyLeftPaddingTotal
	body := sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
	return joinToolParts(header, body)
}
