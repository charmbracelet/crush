package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// -----------------------------------------------------------------------------
// View Tool
// -----------------------------------------------------------------------------

// ViewToolMessageItem is a message item that represents a view tool call.
type ViewToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*ViewToolMessageItem)(nil)

// NewViewToolMessageItem creates a new [ViewToolMessageItem].
func NewViewToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &ViewToolRenderContext{}, canceled)
}

// ViewToolRenderContext renders view tool messages.
type ViewToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (v *ViewToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if !opts.ToolCall.Finished && !opts.Canceled {
		return pendingTool(sty, "View", opts.Anim)
	}

	var params tools.ViewParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	file := fsext.PrettyPath(params.FilePath)
	toolParams := []string{file}
	if params.Limit != 0 {
		toolParams = append(toolParams, "limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.Offset != 0 {
		toolParams = append(toolParams, "offset", fmt.Sprintf("%d", params.Offset))
	}

	header := toolHeader(sty, opts.Status(), "View", cappedWidth, opts.Nested, toolParams...)
	if opts.Nested {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.Result == nil {
		return header
	}

	// Handle image content.
	if opts.Result.Data != "" && strings.HasPrefix(opts.Result.MIMEType, "image/") {
		body := toolOutputImageContent(sty, opts.Result.Data, opts.Result.MIMEType)
		return joinToolParts(header, body)
	}

	// Try to get content from metadata first (contains actual file content).
	var meta tools.ViewResponseMetadata
	content := opts.Result.Content
	if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil && meta.Content != "" {
		content = meta.Content
	}

	if content == "" {
		return header
	}

	// Render code content with syntax highlighting.
	body := toolOutputCodeContent(sty, params.FilePath, content, params.Offset, cappedWidth, opts.Expanded)
	return joinToolParts(header, body)
}

// -----------------------------------------------------------------------------
// Write Tool
// -----------------------------------------------------------------------------

// WriteToolMessageItem is a message item that represents a write tool call.
type WriteToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*WriteToolMessageItem)(nil)

// NewWriteToolMessageItem creates a new [WriteToolMessageItem].
func NewWriteToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &WriteToolRenderContext{}, canceled)
}

// WriteToolRenderContext renders write tool messages.
type WriteToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (w *WriteToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if !opts.ToolCall.Finished && !opts.Canceled {
		return pendingTool(sty, "Write", opts.Anim)
	}

	var params tools.WriteParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	file := fsext.PrettyPath(params.FilePath)
	header := toolHeader(sty, opts.Status(), "Write", cappedWidth, opts.Nested, file)
	if opts.Nested {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if params.Content == "" {
		return header
	}

	// Render code content with syntax highlighting.
	body := toolOutputCodeContent(sty, params.FilePath, params.Content, 0, cappedWidth, opts.Expanded)
	return joinToolParts(header, body)
}

// -----------------------------------------------------------------------------
// Edit Tool
// -----------------------------------------------------------------------------

// EditToolMessageItem is a message item that represents an edit tool call.
type EditToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*EditToolMessageItem)(nil)

// NewEditToolMessageItem creates a new [EditToolMessageItem].
func NewEditToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &EditToolRenderContext{}, canceled)
}

// EditToolRenderContext renders edit tool messages.
type EditToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (e *EditToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	// Edit tool uses full width for diffs.
	if !opts.ToolCall.Finished && !opts.Canceled {
		return pendingTool(sty, "Edit", opts.Anim)
	}

	var params tools.EditParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, width)
	}

	file := fsext.PrettyPath(params.FilePath)
	header := toolHeader(sty, opts.Status(), "Edit", width, opts.Nested, file)
	if opts.Nested {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, width); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.Result == nil {
		return header
	}

	// Get diff content from metadata.
	var meta tools.EditResponseMetadata
	if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err != nil {
		bodyWidth := width - toolBodyLeftPaddingTotal
		body := sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.Expanded))
		return joinToolParts(header, body)
	}

	// Render diff.
	body := toolOutputDiffContent(sty, file, meta.OldContent, meta.NewContent, width, opts.Expanded)
	return joinToolParts(header, body)
}

// -----------------------------------------------------------------------------
// MultiEdit Tool
// -----------------------------------------------------------------------------

// MultiEditToolMessageItem is a message item that represents a multi-edit tool call.
type MultiEditToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*MultiEditToolMessageItem)(nil)

// NewMultiEditToolMessageItem creates a new [MultiEditToolMessageItem].
func NewMultiEditToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &MultiEditToolRenderContext{}, canceled)
}

// MultiEditToolRenderContext renders multi-edit tool messages.
type MultiEditToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (m *MultiEditToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	// MultiEdit tool uses full width for diffs.
	if !opts.ToolCall.Finished && !opts.Canceled {
		return pendingTool(sty, "Multi-Edit", opts.Anim)
	}

	var params tools.MultiEditParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, width)
	}

	file := fsext.PrettyPath(params.FilePath)
	toolParams := []string{file}
	if len(params.Edits) > 0 {
		toolParams = append(toolParams, "edits", fmt.Sprintf("%d", len(params.Edits)))
	}

	header := toolHeader(sty, opts.Status(), "Multi-Edit", width, opts.Nested, toolParams...)
	if opts.Nested {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, width); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.Result == nil {
		return header
	}

	// Get diff content from metadata.
	var meta tools.MultiEditResponseMetadata
	if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err != nil {
		bodyWidth := width - toolBodyLeftPaddingTotal
		body := sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.Expanded))
		return joinToolParts(header, body)
	}

	// Render diff with optional failed edits note.
	body := toolOutputMultiEditDiffContent(sty, file, meta, len(params.Edits), width, opts.Expanded)
	return joinToolParts(header, body)
}
