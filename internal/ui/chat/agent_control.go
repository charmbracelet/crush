package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// NewAgentControlToolMessageItem creates the item for the tools that talk to
// a running sub-agent: agent_send, agent_status, and agent_stop. They name
// the sub-agent the way its dispatch does, rather than dumping their JSON.
func NewAgentControlToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &AgentControlToolRenderContext{}, canceled)
}

// AgentControlToolRenderContext renders agent_send, agent_status, and
// agent_stop calls.
type AgentControlToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (r *AgentControlToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	name := humanizedToolName(opts.ToolCall.Name)
	if opts.IsPending() {
		return pendingTool(sty, name, opts.Anim, opts.Compact)
	}

	label, prose, showResult := agentControlParts(opts.ToolCall)
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
	if showResult && opts.HasResult() && opts.Result.Content != "" {
		body := renderToolResultTextContent(sty, opts.Result.Content, toolResultContentWidths{Body: bodyWidth, Diff: cappedWidth}, opts.ExpandedContent)
		return joinToolParts(header, body)
	}
	return header
}

// agentControlParts reads which sub-agent a call addresses and the prose it
// carries: the message sent, or the reason for a stop. A status call has no
// prose of its own, so its result is what is worth showing.
func agentControlParts(tc message.ToolCall) (label, prose string, showResult bool) {
	switch tc.Name {
	case agent.AgentSendToolName:
		var params agent.AgentSendParams
		_ = json.Unmarshal([]byte(tc.Input), &params)
		return params.Label, params.Message, false
	case agent.AgentStopToolName:
		var params agent.AgentStopParams
		_ = json.Unmarshal([]byte(tc.Input), &params)
		return params.Label, params.Reason, false
	default:
		var params agent.AgentStatusParams
		_ = json.Unmarshal([]byte(tc.Input), &params)
		return params.Label, "", true
	}
}

// agentMessageContent wraps prose addressed to a sub-agent, collapsing it
// the way tool output collapses.
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
