package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/charmbracelet/fantasy/ai"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/agent_tool.md
var agentToolDescription []byte

type AgentParams struct {
	Prompt string `json:"prompt" description:"The task for the agent to perform"`
}

const (
	AgentToolName = "agent"
)

func (c *coordinator) agentTool() (ai.AgentTool, error) {
	agentCfg, ok := c.cfg.Agents[config.AgentTask]
	if !ok {
		return nil, errors.New("task agent not configured")
	}
	prompt, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	agent, err := c.buildAgent(prompt, agentCfg)
	if err != nil {
		return nil, err
	}
	return ai.NewAgentTool(
		AgentToolName,
		string(agentToolDescription),
		func(ctx context.Context, params AgentParams, call ai.ToolCall) (ai.ToolResponse, error) {
			if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
				return ai.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
			}
			if params.Prompt == "" {
				return ai.NewTextErrorResponse("prompt is required"), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return ai.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return ai.ToolResponse{}, errors.New("agent message id missing from context")
			}

			agentToolSessionID := c.sessions.CreateAgentToolSessionID(agentMessageID, call.ID)
			session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, sessionID, "New Agent Session")
			if err != nil {
				return ai.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
			}
			model := agent.Model()
			maxTokens := model.CatwalkCfg.DefaultMaxTokens
			if model.ModelCfg.MaxTokens != 0 {
				maxTokens = model.ModelCfg.MaxTokens
			}

			providerCfg, ok := c.cfg.Providers.Get(model.ModelCfg.Provider)
			if !ok {
				return ai.ToolResponse{}, errors.New("model provider not configured")
			}
			result, err := agent.Run(ctx, SessionAgentCall{
				SessionID:        session.ID,
				Prompt:           params.Prompt,
				MaxOutputTokens:  maxTokens,
				ProviderOptions:  getProviderOptions(model, providerCfg.Type),
				Temperature:      model.ModelCfg.Temperature,
				TopP:             model.ModelCfg.TopP,
				TopK:             model.ModelCfg.TopK,
				FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
				PresencePenalty:  model.ModelCfg.PresencePenalty,
			})
			if err != nil {
				return ai.NewTextErrorResponse("error generating response"), nil
			}
			updatedSession, err := c.sessions.Get(ctx, session.ID)
			if err != nil {
				return ai.ToolResponse{}, fmt.Errorf("error getting session: %s", err)
			}
			parentSession, err := c.sessions.Get(ctx, sessionID)
			if err != nil {
				return ai.ToolResponse{}, fmt.Errorf("error getting parent session: %s", err)
			}

			parentSession.Cost += updatedSession.Cost

			_, err = c.sessions.Save(ctx, parentSession)
			if err != nil {
				return ai.ToolResponse{}, fmt.Errorf("error saving parent session: %s", err)
			}
			return ai.NewTextResponse(result.Response.Content.Text()), nil
		}), nil
}
