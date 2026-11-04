package agent

import (
	"context"
	_ "embed"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
)

//go:embed templates/agent_stop.md
var agentStopDescription string

// AgentStopToolName is the name of the tool that stops a background
// subagent.
const AgentStopToolName = "agent_stop"

// AgentStopParams is the input to the agent_stop tool.
type AgentStopParams struct {
	Name string `json:"name" description:"The name of the background agent, as returned when it was started"`
}

func (c *coordinator) agentStopTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		AgentStopToolName,
		agentStopDescription,
		func(ctx context.Context, params AgentStopParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Name == "" {
				return fantasy.NewTextErrorResponse("name is required"), nil
			}
			return fantasy.NewTextResponse(c.stopBackground(tools.GetSessionFromContext(ctx), params.Name)), nil
		},
	)
}
