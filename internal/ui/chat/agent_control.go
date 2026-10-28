package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/x/ansi"

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

// AgentControlToolRenderContext renders calls addressed to a subagent.
type AgentControlToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (r *AgentControlToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	name := prettifyToolName(opts.ToolCall.Name)
	if opts.IsPending() {
		return pendingTool(sty, name, opts.Anim, opts.Compact)
	}

	label, prose := agentControlParts(opts.ToolCall)
	var params []string
	if label != "" {
		params = append(params, sty.Tool.ParamMain.Render("→ ")+sty.Tool.AgentName.Render(agent.SubagentName(label)))
	}
	header := toolHeader(sty, opts.Status, name, cappedWidth, opts, params...)
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
	if rows := agentStatusRows(sty, opts.Result, bodyWidth); rows != "" {
		return joinToolParts(header, sty.Tool.Body.Render(rows))
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

// agentStatusRows draws an agent_status result as one aligned row per
// subagent, or returns "" when the result carries no status metadata.
func agentStatusRows(sty *styles.Styles, result *message.ToolResult, width int) string {
	var meta agent.AgentStatusResponseMetadata
	if result == nil || json.Unmarshal([]byte(result.Metadata), &meta) != nil || len(meta.Agents) == 0 {
		return ""
	}
	names := make([]string, len(meta.Agents))
	nameWidth := 0
	for i, a := range meta.Agents {
		names[i] = a.Label
		nameWidth = max(nameWidth, lipgloss.Width(names[i]))
	}
	rows := make([]string, len(meta.Agents))
	for i, a := range meta.Agents {
		state := string(a.State)
		if a.Detail != "" {
			state += " (" + a.Detail + ")"
		}
		if a.Timing != "" {
			state += " " + a.Timing
		}
		facts := []string{fmt.Sprintf("%d tool calls", a.ToolCalls)}
		if a.Access == "write" {
			facts = append(facts, "can write")
		}
		if a.Cost > 0 {
			facts = append(facts, fmt.Sprintf("$%.2f", a.Cost))
		}
		row := fmt.Sprintf("%s %s  %s %s",
			agentStateIcon(sty, a.State),
			sty.Tool.AgentName.Render(names[i]+strings.Repeat(" ", nameWidth-lipgloss.Width(names[i]))),
			sty.Tool.AgentMessage.Render(state),
			sty.Tool.ParamMain.Render("· "+strings.Join(facts, " · ")),
		)
		rows[i] = ansi.Truncate(row, width, "…")
	}
	return strings.Join(rows, "\n")
}

func agentStateIcon(sty *styles.Styles, state agent.SubagentState) string {
	switch state {
	case agent.SubagentFinished:
		return toolIcon(sty, ToolStatusSuccess)
	case agent.SubagentRunning, agent.SubagentStopping:
		return toolIcon(sty, ToolStatusRunning)
	case agent.SubagentStopped:
		return toolIcon(sty, ToolStatusCanceled)
	default:
		return toolIcon(sty, ToolStatusError)
	}
}
