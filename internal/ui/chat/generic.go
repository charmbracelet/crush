package chat

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/joestump-agent/a2tea"
	"github.com/joestump-agent/a2tea/render"
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

// RenderTool implements the [ToolRenderer] interface.
func (g *GenericToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	name := humanizedToolName(opts.ToolCall.Name)

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

	// If the tool result contains an A2UI surface (e.g. a read_mcp_resource
	// that returned application/a2ui+json), render it as a live surface
	// instead of raw text. This is what makes cairn://artifact/{id}/a2ui
	// and similar resource reads render inline — no JSON barf.
	if a2tea.Contains(opts.Result.Content) {
		surf, err := renderToolA2UI(sty, opts.Result.Content, bodyWidth)
		if err == nil && surf != "" {
			return joinToolParts(header, surf)
		}
	}

	body := renderToolResultTextContent(sty, opts.Result.Content, toolResultContentWidths{Body: bodyWidth, Diff: cappedWidth}, opts.ExpandedContent)
	return joinToolParts(header, body)
}

// renderToolA2UI renders a static (non-interactive) A2UI surface from tool
// result content. Unlike the live surfaces on AssistantMessageItem, this does
// not support focus/button interaction — it renders a snapshot of the surface
// using the same themed styles.
func renderToolA2UI(sty *styles.Styles, content string, width int) (string, error) {
	parts, err := a2tea.Scan(content)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	themedStyles := a2uiThemeStyles(sty)
	for _, p := range parts {
		if len(p.Messages) == 0 {
			continue
		}
		model, err := a2tea.Render(p.Messages, render.WithStyles(themedStyles))
		if err != nil {
			continue
		}
		if rm, ok := model.(render.Model); ok {
			rm.SetSize(width, 0)
			b.WriteString(strings.TrimRight(rm.View().Content, "\n"))
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
