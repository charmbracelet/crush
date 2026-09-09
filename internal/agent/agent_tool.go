package agent

import (
	"context"
	_ "embed"
	"errors"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/agent_tool.md
var agentToolDescription string

type AgentParams struct {
	Prompt string `json:"prompt" description:"The task for the agent to perform"`
	SubagentType string `json:"subagent_type,omitempty" description:"Type of sub-agent to spawn, e.g. general-purpose"`
}

const (
	AgentToolName = "agent"
)

func (c *coordinator) agentTool(_ctx context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		AgentToolName,
		agentToolDescription,
		func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			agentID := config.AgentTask
			if params.SubagentType == "general-purpose" {
				agentID = config.AgentCoder
			}
			agentCfg, ok := c.cfg.Config().Agents[agentID]
			if !ok {
				agentCfg, ok = c.cfg.Config().Agents[config.AgentTask]
				if !ok {
					return fantasy.ToolResponse{}, errors.New("task agent not configured")
				}
			}

			promptObj, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			agent, err := c.buildAgent(ctx, promptObj, agentCfg, true)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			return c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   "New Agent Session",
			})
		},
	), nil
}
