package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagent"
)

func (c *coordinator) subagentTool(ctx context.Context, sub *subagent.Subagent) (fantasy.AgentTool, error) {
	// 1. Build allowed tools list
	allowedTools := c.resolveSubagentTools(sub)

	// 2. Select model
	modelType := c.resolveSubagentModel(sub)

	// 3. Build agent config
	agentCfg := config.Agent{
		ID:           "subagent-" + sub.Name,
		Name:         sub.Name,
		Description:  sub.Description,
		Model:        modelType,
		AllowedTools: allowedTools,
		// TODO: resolve MCPs
	}

	// 4. Create agent with custom system prompt
	// We need to build the agent with the custom prompt from the subagent file
	agent, err := c.buildAgentWithSystemPrompt(ctx, sub.SystemPrompt, agentCfg, true)
	if err != nil {
		return nil, err
	}

	// 5. Return as parallel tool (concurrent execution)
	return fantasy.NewParallelAgentTool(
		sub.Name,
		sub.Description,
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

			agentToolSessionID := c.sessions.CreateAgentToolSessionID(agentMessageID, call.ID)
			title := fmt.Sprintf("Subagent: %s", sub.Name)
			session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, sessionID, title)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
			}

			// Handle permission modes
			switch sub.PermissionMode {
			case "bypassPermissions":
				c.permissions.AutoApproveSession(session.ID)
			case "acceptEdits":
				// TODO: handle acceptEdits (auto-approve only edit/write)
				// For now, auto-approve all for MVP if acceptEdits is requested
				c.permissions.AutoApproveSession(session.ID)
			case "dontAsk":
				// TODO: handle dontAsk (auto-deny)
			}

			model := agent.Model()
			maxTokens := model.CatwalkCfg.DefaultMaxTokens
			if model.ModelCfg.MaxTokens != 0 {
				maxTokens = model.ModelCfg.MaxTokens
			}

			providerCfg, ok := c.cfg.Providers.Get(model.ModelCfg.Provider)
			if !ok {
				return fantasy.ToolResponse{}, errors.New("model provider not configured")
			}

			publishSubagentStarted(sub.Name, sub.Color)
			defer publishSubagentStopped()

			result, err := agent.Run(ctx, SessionAgentCall{
				SessionID:        session.ID,
				Prompt:           params.Prompt,
				MaxOutputTokens:  maxTokens,
				ProviderOptions:  getProviderOptions(model, providerCfg),
				Temperature:      model.ModelCfg.Temperature,
				TopP:             model.ModelCfg.TopP,
				TopK:             model.ModelCfg.TopK,
				FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
				PresencePenalty:  model.ModelCfg.PresencePenalty,
			})
			if err != nil {
				slog.Error("subagent execution failed", "subagent", sub.Name, "error", err)
				return fantasy.NewTextErrorResponse("error generating response"), nil
			}

			updatedSession, err := c.sessions.Get(ctx, session.ID)
			if err == nil {
				if parentSession, err := c.sessions.Get(ctx, sessionID); err == nil {
					parentSession.Cost += updatedSession.Cost
					_, _ = c.sessions.Save(ctx, parentSession)
				}
			}

			return fantasy.NewTextResponse(result.Response.Content.Text()), nil
		},
	), nil
}

func (c *coordinator) resolveSubagentTools(sub *subagent.Subagent) []string {
	var allowed []string
	if sub.Tools != nil {
		// Map Claude tool names to Crush equivalents
		for _, t := range sub.Tools {
			switch t {
			case "Read":
				allowed = append(allowed, tools.ViewToolName)
			case "Glob":
				allowed = append(allowed, tools.GlobToolName)
			case "Grep":
				allowed = append(allowed, tools.GrepToolName)
			case "Bash":
				allowed = append(allowed, tools.BashToolName)
			case "Write":
				allowed = append(allowed, tools.WriteToolName)
			case "Edit":
				allowed = append(allowed, tools.EditToolName)
			default:
				allowed = append(allowed, t)
			}
		}
	} else {
		// Inherit all tools
		allowed = config.AllToolNames()
	}

	if sub.DisallowedTools != nil {
		var filtered []string
		for _, t := range allowed {
			disallowed := false
			for _, d := range sub.DisallowedTools {
				if t == d || (d == "Read" && t == tools.ViewToolName) ||
					(d == "Glob" && t == tools.GlobToolName) ||
					(d == "Grep" && t == tools.GrepToolName) ||
					(d == "Bash" && t == tools.BashToolName) ||
					(d == "Write" && t == tools.WriteToolName) ||
					(d == "Edit" && t == tools.EditToolName) {
					disallowed = true
					break
				}
			}
			if !disallowed {
				filtered = append(filtered, t)
			}
		}
		allowed = filtered
	}

	return allowed
}

func (c *coordinator) resolveSubagentModel(sub *subagent.Subagent) config.SelectedModelType {
	switch sub.Model {
	case "haiku", "small":
		return config.SelectedModelTypeSmall
	case "opus", "sonnet", "large", "":
		return config.SelectedModelTypeLarge
	case "inherit":
		// Use parent's model (currentAgent's model)
		// Default to large for now as we don't easily know the parent model here without more context
		return config.SelectedModelTypeLarge
	default:
		return config.SelectedModelTypeLarge
	}
}

func (c *coordinator) buildAgentWithSystemPrompt(ctx context.Context, systemPrompt string, agent config.Agent, isSubAgent bool) (SessionAgent, error) {
	large, small, err := c.buildAgentModels(ctx, isSubAgent)
	if err != nil {
		return nil, err
	}

	largeProviderCfg, _ := c.cfg.Providers.Get(large.ModelCfg.Provider)
	result := NewSessionAgent(SessionAgentOptions{
		LargeModel:           large,
		SmallModel:           small,
		SystemPromptPrefix:   largeProviderCfg.SystemPromptPrefix,
		SystemPrompt:         systemPrompt, // Use the provided prompt directly
		IsSubAgent:           isSubAgent,
		DisableAutoSummarize: c.cfg.Options.DisableAutoSummarize,
		IsYolo:               c.permissions.SkipRequests(),
		Sessions:             c.sessions,
		Messages:             c.messages,
		Tools:                nil,
	})

	// Still need to build tools
	c.readyWg.Go(func() error {
		tools, err := c.buildTools(ctx, agent, isSubAgent)
		if err != nil {
			return err
		}
		result.SetTools(tools)
		return nil
	})

	return result, nil
}
