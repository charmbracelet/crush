package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
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
		// toolOutputImageContent applies sty.Tool.Body itself — wrapping it
		// again here double-indented image results.
		return joinToolParts(header, toolOutputImageContent(sty, opts.Result.Data, opts.Result.MIMEType))
	}

	widths := toolResultContentWidths{Body: bodyWidth, Diff: cappedWidth}

	// If the tool result carries an A2UI surface (a read_mcp_resource that
	// returned application/a2ui+json), render it as a surface snapshot
	// instead of raw text. The payload arrives via response metadata — the
	// model only sees a placeholder, so it cannot echo the JSON back and
	// double-render the surface. This is what makes
	// cairn://artifact/{id}/a2ui and similar resource reads render inline.
	if opts.Result.Metadata != "" {
		var meta tools.ReadMCPResourceResponseMetadata
		if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil && len(meta.A2UISurfaces) > 0 {
			return joinToolParts(header, renderToolA2UIResultBody(sty, meta.A2UISurfaces, opts, widths))
		}
	}
	// Pre-change read_mcp_resource results persisted the raw payload in the
	// content itself — scan for it so old sessions still render as surfaces.
	// Scoped to this tool (crush_logs and friends share this renderer and
	// must keep payload-shaped text as text), and gated on contentHasA2UI so
	// A2UI examples inside markdown code stay code (#6, #96).
	if opts.ToolCall.Name == tools.ReadMCPResourceToolName && contentHasA2UI(opts.Result.Content) {
		surf, err := renderToolA2UI(sty, opts.Result.Content, bodyWidth)
		if err == nil && surf != "" {
			return joinToolParts(header, sty.Tool.Body.Render(clampToolA2UI(sty, surf, bodyWidth, opts.ExpandedContent)))
		}
	}

	body := renderToolResultTextContent(sty, opts.Result.Content, widths, opts.ExpandedContent)
	return joinToolParts(header, body)
}

// renderToolA2UIResultBody renders the metadata-delivered surfaces plus any
// real text content the resource returned alongside them. When every surface
// fails to render, an alert replaces the body — the model-facing placeholder
// claims the user can already see the surface, which would be false here.
func renderToolA2UIResultBody(sty *styles.Styles, surfaces []string, opts *ToolRenderOpts, widths toolResultContentWidths) string {
	var b strings.Builder
	failed := 0
	for _, surf := range surfaces {
		rendered, err := renderToolA2UI(sty, surf, widths.Body)
		if err != nil || rendered == "" {
			failed++
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(rendered)
	}

	var chunks []string
	if b.Len() > 0 {
		chunks = append(chunks, sty.Tool.Body.Render(clampToolA2UI(sty, b.String(), widths.Body, opts.ExpandedContent)))
	}
	// Alert on ANY failed surface, not only when all fail: a sibling
	// surface rendering fine must not hide that another one vanished —
	// the model was told the user can see every one of them.
	if failed > 0 {
		chunks = append(chunks, sty.Tool.Body.Render(renderToolA2UIAlert(sty, widths.Body)))
	}
	// Show any real text the resource returned alongside its surfaces — the
	// model-facing placeholder lines are stripped, the rest belongs to the
	// user just as much as the surfaces do.
	if text := stripA2UIPlaceholders(opts.Result.Content); text != "" {
		chunks = append(chunks, renderToolResultTextContent(sty, text, widths, opts.ExpandedContent))
	}
	return strings.Join(chunks, "\n")
}

// stripA2UIPlaceholders removes the model-facing surface placeholder lines
// from tool content, leaving only text the user should see.
func stripA2UIPlaceholders(content string) string {
	var out []string
	for _, ln := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), tools.A2UISurfacePlaceholderPrefix) {
			continue
		}
		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// clampToolA2UI applies the collapsed-height budget every other tool body
// gets from responseContextHeight. The surface is pre-rendered ANSI, so the
// clamp slices whole lines and appends the standard truncation footer;
// expanding the tool result restores the full surface.
func clampToolA2UI(sty *styles.Styles, body string, width int, expanded bool) string {
	lines := strings.Split(body, "\n")
	if expanded || len(lines) <= responseContextHeight {
		return body
	}
	out := append([]string{}, lines[:responseContextHeight]...)
	out = append(out, sty.Tool.ContentTruncation.
		Width(width).
		Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-responseContextHeight)))
	return strings.Join(out, "\n")
}

// renderToolA2UIAlert mirrors the assistant path's renderA2UIAlert for tool
// results: the resource advertised A2UI but a2tea could not draw a surface.
func renderToolA2UIAlert(sty *styles.Styles, width int) string {
	inner := max(width-2, 1)
	tag := sty.Messages.ErrorTag.Render("A2UI")
	title := sty.Messages.ErrorTitle.Render("couldn't render this resource's UI surface")
	reason := sty.Messages.ErrorDetails.Width(inner).Render(
		"The A2UI content was malformed or used unsupported components.")
	return tag + " " + title + "\n\n" + reason
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
	proseStyle := lipgloss.NewStyle().Width(width)
	for _, p := range parts {
		// Keep each part's prose, wrapped to the body budget: pre-change
		// persisted results interleave text with surfaces, and dropping
		// (or overflowing) it would corrupt content the model saw.
		if text := strings.TrimSpace(p.Text); text != "" {
			b.WriteString(proseStyle.Render(text))
			b.WriteString("\n")
		}
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
