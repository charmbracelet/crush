package agent

import (
	"context"
	_ "embed"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
)

//go:embed templates/agent_send.md
var agentSendDescription string

// AgentSendToolName is the name of the tool that messages a background
// subagent.
const AgentSendToolName = "agent_send"

// AgentSendParams is the input to the agent_send tool.
type AgentSendParams struct {
	Name    string `json:"name" description:"The name of the background agent, as returned when it was started"`
	Message string `json:"message" description:"What to tell the agent"`
}

func (c *coordinator) agentSendTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		AgentSendToolName,
		agentSendDescription,
		func(ctx context.Context, params AgentSendParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Name == "" || params.Message == "" {
				return fantasy.NewTextErrorResponse("name and message are required"), nil
			}
			text, err := c.sendToBackground(ctx, tools.GetSessionFromContext(ctx), params.Name, params.Message)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(text), nil
		},
	)
}
