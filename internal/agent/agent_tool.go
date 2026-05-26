package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/agent_tool.md
var agentToolDescription string

type AgentParams struct {
	Prompt string `json:"prompt" description:"The task for the agent to perform"`
	Task   string `json:"task" description:"Type of agent to use: task (general delegated work) or explore (fast read-only codebase search). Default: task"`
}

const (
	AgentToolName    = "agent"
	AgentTaskExplore = "explore"
)

func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
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

			task := params.Task
			if task == "" {
				task = "task"
			}

			var agentCfg config.Agent
			var promptStr *prompt.Prompt
			switch task {
			case AgentTaskExplore:
				var ok bool
				agentCfg, ok = c.cfg.Config().Agents[config.AgentExplore]
				if !ok {
					return fantasy.NewTextErrorResponse("explore agent not configured"), nil
				}
				var err error
				promptStr, err = explorePrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("building explore prompt: %w", err)
				}
			default:
				var ok bool
				agentCfg, ok = c.cfg.Config().Agents[config.AgentTask]
				if !ok {
					return fantasy.NewTextErrorResponse("task agent not configured"), nil
				}
				var err error
				promptStr, err = taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("building task prompt: %w", err)
				}
			}

			agent, err := c.buildAgent(ctx, promptStr, agentCfg, true)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("building agent: %w", err)
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
