package chat

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// GenericToolMessageItem is a message item that represents an unknown tool call.
type GenericToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*GenericToolMessageItem)(nil)

// NewGenericToolMessageItem creates a new [GenericToolMessageItem].
func NewGenericToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &GenericToolRenderContext{}, canceled)
}

// GenericToolRenderContext renders unknown/generic tool messages.
type GenericToolRenderContext struct{}

// CustomToolLookup finds a plugin tool's declaration by name. The UI model
// sets it so calls render with the tool's own title and summary; nil means
// every call renders plainly.
var CustomToolLookup func(name string) (config.CustomToolConfig, bool)

// customToolHeader returns a plugin tool's title and summary for a call.
func customToolHeader(call message.ToolCall) (title, summary string, ok bool) {
	if CustomToolLookup == nil {
		return "", "", false
	}
	cfg, found := CustomToolLookup(call.Name)
	if !found {
		return "", "", false
	}
	return tools.CustomToolDisplay(cfg, call.Name, call.Input)
}

// RenderTool implements the [ToolRenderer] interface.
func (g *GenericToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	name := humanizedToolName(opts.ToolCall.Name)
	title, summary, custom := customToolHeader(opts.ToolCall)
	if custom && title != "" {
		name = title
	}

	if opts.IsPending() {
		return pendingTool(sty, name, opts.Anim, opts.Compact)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var toolParams []string
	switch {
	case custom:
		if summary != "" {
			toolParams = append(toolParams, summary)
		}
	case len(params) > 0:
		parsed, _ := json.Marshal(params)
		toolParams = append(toolParams, string(parsed))
	}

	header := toolHeader(sty, opts.Status, name, cappedWidth, opts, toolParams...)
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

	if opts.Result.Data != "" && strings.HasPrefix(opts.Result.MIMEType, "image/") {
		body := sty.Tool.Body.Render(toolOutputImageContent(sty, opts.Result.Data, opts.Result.MIMEType))
		return joinToolParts(header, body)
	}

	body := renderToolResultTextContent(sty, opts.Result.Content, toolResultContentWidths{Body: bodyWidth, Diff: cappedWidth}, opts.ExpandedContent)
	return joinToolParts(header, body)
}
