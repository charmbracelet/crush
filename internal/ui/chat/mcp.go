package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// MCPToolMessageItem is a message item that represents a bash tool call.
type MCPToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*MCPToolMessageItem)(nil)

// NewMCPToolMessageItem creates a new [MCPToolMessageItem].
func NewMCPToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &MCPToolRenderContext{}, canceled)
}

// MCPToolRenderContext renders bash tool messages.
type MCPToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (b *MCPToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	toolNameParts := strings.SplitN(opts.ToolCall.Name, "_", 3)
	if len(toolNameParts) != 3 {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid tool name"}, cappedWidth)
	}
	mcpName := humanizedToolName(toolNameParts[1])
	toolName := humanizedToolName(toolNameParts[2])

	mcpName = sty.Tool.MCPName.Render(mcpName)
	toolName = sty.Tool.MCPToolName.Render(toolName)

	name := fmt.Sprintf("%s %s %s", mcpName, sty.Tool.MCPArrow.String(), toolName)

	if opts.IsPending() {
		return pendingTool(sty, name, opts.Anim, opts.Compact)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var toolParams []string
	if len(params) > 0 {
		parsed, _ := json.Marshal(params)
		toolParams = append(toolParams, string(parsed))
	}

	header := toolHeader(sty, opts.Status, name, cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() || opts.Result.Content == "" {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	body := renderToolResultTextContent(sty, opts.Result.Content, toolResultContentWidths{Body: bodyWidth, Diff: cappedWidth}, opts.ExpandedContent)
	return joinToolParts(header, body)
}
