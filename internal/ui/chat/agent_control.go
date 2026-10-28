package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// NewAgentControlToolMessageItem renders agent_send, agent_status, and agent_stop.
func NewAgentControlToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &AgentControlToolRenderContext{}, canceled)
}

// AgentControlToolRenderContext renders calls addressed to a sub-agent.
type AgentControlToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (r *AgentControlToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	name := humanizedToolName(opts.ToolCall.Name)
	if opts.IsPending() {
		return pendingTool(sty, name, opts.Anim, opts.Compact)
	}

	label, prose := agentControlParts(opts.ToolCall)
	target := sty.Tool.ParamMain.Render("all")
	if label != "" {
		target = sty.Tool.AgentName.Render(SubagentDisplayName(label))
	}
	header := toolHeader(sty, opts.Status, name, cappedWidth, opts, sty.Tool.ParamMain.Render("→ ")+target)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	if prose != "" {
		return joinToolParts(header, sty.Tool.Body.Render(agentMessageContent(sty, prose, bodyWidth, opts.ExpandedContent)))
	}
	if opts.HasResult() && opts.Result.Content != "" {
		body := renderToolResultTextContent(sty, opts.Result.Content, toolResultContentWidths{Body: bodyWidth, Diff: cappedWidth}, opts.ExpandedContent)
		return joinToolParts(header, body)
	}
	return header
}

// agentControlParts returns the addressed label and the prose to show: a message, a stop reason, or none.
func agentControlParts(tc message.ToolCall) (label, prose string) {
	var params struct {
		Label   string `json:"label"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	_ = json.Unmarshal([]byte(tc.Input), &params)
	return params.Label, params.Message + params.Reason
}

// agentMessageContent wraps prose and collapses it like tool output.
func agentMessageContent(sty *styles.Styles, text string, width int, expanded bool) string {
	width = min(width, maxTextWidth)
	lines := strings.Split(sty.Tool.AgentMessage.Width(width).Render(strings.TrimSpace(text)), "\n")
	maxLines := collapsedMaxLines(len(lines))
	if expanded || len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	hidden := sty.Tool.ContentTruncation.Width(width).
		Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-maxLines))
	return strings.Join(append(lines[:maxLines], hidden), "\n")
}
