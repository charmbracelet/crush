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

// AgentParams holds the parameters for the agent tool.
type AgentParams struct {
	Prompt string `json:"prompt" description:"The task for the agent to perform"`
	// WriteTools gives the agent tools that can modify files and run
	// commands (bash, edit, multiedit, write, download). Without it the
	// agent is read-only.
	WriteTools bool `json:"write_tools,omitempty" description:"Give the agent tools that can modify files and run commands. By default the agent only has read-only tools"`
	// Provider and Model override the model the sub-agent runs on.
	// Provider requires Model; Model alone reuses the currently selected
	// model's provider. Both empty runs the selected model.
	Provider string `json:"provider,omitempty" description:"Optional provider ID to run the agent with instead of the selected model's provider; requires model"`
	Model    string `json:"model,omitempty" description:"Optional model ID to run the agent with instead of the selected model"`
}

const (
	AgentToolName = "agent"
)

// subAgentKey identifies a cached sub-agent: whether it has write tools
// plus the provider/model pair it runs on. The zero pair means the
// currently selected sub-agent model.
type subAgentKey struct {
	Write    bool
	Provider string
	Model    string
}

func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
	readCfg, ok := c.cfg.Config().Agents[config.AgentTask]
	if !ok {
		return nil, errors.New("task agent not configured")
	}
	writeCfg, ok := c.cfg.Config().Agents[config.AgentTaskWrite]
	if !ok {
		return nil, errors.New("task write agent not configured")
	}
	tpl, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	readAgent, err := c.buildAgent(ctx, tpl, readCfg, true)
	if err != nil {
		return nil, err
	}
	writeAgent, err := c.buildAgent(ctx, tpl, writeCfg, true)
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

			agentCfg := readCfg
			defaultAgent := readAgent
			if params.WriteTools {
				agentCfg = writeCfg
				defaultAgent = writeAgent
			}

			agent := defaultAgent
			if params.Provider != "" || params.Model != "" {
				var err error
				agent, err = c.overrideSubAgent(ctx, tpl, agentCfg, params.Provider, params.Model)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
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

// overrideSubAgent resolves the sub-agent to run for an agent-tool model
// override, building and caching it on first use. Model without provider
// reuses the selected sub-agent model's provider; provider without model
// is rejected.
func (c *coordinator) overrideSubAgent(ctx context.Context, tpl *prompt.Prompt, agentCfg config.Agent, providerID, modelID string) (SessionAgent, error) {
	if providerID == "" {
		providerID = c.subAgentModels().Large.ModelCfg.Provider
	}
	if modelID == "" {
		return nil, fmt.Errorf("provider %q requires a model", providerID)
	}

	key := subAgentKey{Write: agentCfg.ID == config.AgentTaskWrite, Provider: providerID, Model: modelID}

	c.subAgentMu.Lock()
	if cached, ok := c.subAgentCache[key]; ok {
		c.subAgentMu.Unlock()
		return cached, nil
	}
	gen := c.subAgentGen
	c.subAgentMu.Unlock()

	// Build outside the lock: a concurrent call for the same pair may
	// build its own agent, and the last writer wins the cache entry.
	agent, err := c.buildOverrideSubAgent(ctx, tpl, agentCfg, providerID, modelID)
	if err != nil {
		return nil, err
	}

	// Publish only if no rebuild happened meanwhile: a rebuild means the
	// models this agent captured may already be stale.
	c.subAgentMu.Lock()
	if c.subAgentGen == gen {
		if c.subAgentCache == nil {
			c.subAgentCache = make(map[subAgentKey]SessionAgent)
		}
		c.subAgentCache[key] = agent
	}
	c.subAgentMu.Unlock()

	return agent, nil
}

// buildOverrideSubAgent builds a sub-agent whose large model is the
// explicitly requested provider/model pair. Unlike buildAgent, which
// defers prompt and tool construction to readiness goroutines, everything
// is built synchronously because the agent runs immediately after this
// returns. The small model stays shared so summarization keeps using it.
func (c *coordinator) buildOverrideSubAgent(ctx context.Context, tpl *prompt.Prompt, agentCfg config.Agent, providerID, modelID string) (SessionAgent, error) {
	if c.models.Get().large.Model == nil {
		if err := c.rebuildModels(ctx); err != nil {
			return nil, err
		}
	}

	large, err := c.buildModelOverride(ctx, providerID, modelID, true)
	if err != nil {
		return nil, err
	}
	models := func() ModelSet {
		return ModelSet{Large: large, Small: c.models.Get().small}
	}

	systemPrompt, err := tpl.Build(ctx, large.Model.Provider(), large.Model.Model(), c.cfg)
	if err != nil {
		return nil, err
	}

	subTools, err := c.buildTools(ctx, agentCfg, true)
	if err != nil {
		return nil, err
	}

	opts := c.newSessionAgentOptions(models, true)
	opts.SystemPrompt = systemPrompt
	opts.Tools = subTools
	return NewSessionAgent(opts), nil
}
