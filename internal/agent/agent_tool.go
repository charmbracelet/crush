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
}

const (
	AgentToolName = "agent"
)

func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
	agentCfg, ok := c.cfg.Config().Agents[config.AgentTask]
	if !ok {
		return nil, errors.New("task agent not configured")
	}
	promptTemplate, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	// The sub-agent carries no system prompt of its own: one instance
	// serves every parent run, and each delegated turn renders its own
	// prompt below for the model it inherited.
	agent, ready, err := c.buildAgent(ctx, nil, agentCfg, true)
	if err != nil {
		return nil, err
	}
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

			// The delegated turn runs on the models of the run that
			// asked for it, rebuilt with sub-agent provider settings.
			models, err := c.subAgentModels(ctx)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			// The prompt is rendered for the provider and model this
			// turn will talk to, which is not necessarily the
			// workspace's. It goes on the call rather than on the
			// shared sub-agent instance, so concurrent parent runs
			// cannot overwrite each other's.
			large := models.Large()
			systemPrompt, err := promptTemplate.Build(ctx, large.Model.Provider(), large.Model.Model(), c.cfg)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			return c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				Ready:          ready,
				Models:         models,
				SystemPrompt:   systemPrompt,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   "New Agent Session",
				// The delegated turn is part of this run, so it needs
				// the same approval. Without it a `crush run` blocks
				// forever on the child session's first permission
				// request: nobody is there to answer it.
				AutoApprove: AutoApproveFromContext(ctx),
			})
		},
	), nil
}
