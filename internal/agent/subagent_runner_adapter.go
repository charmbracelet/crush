package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/plugin"
)

// SubAgentRunnerAdapter adapts coordinator sub-agent execution to plugin.SubAgentRunner.
type SubAgentRunnerAdapter struct {
	coordinator *coordinator

	mu sync.RWMutex
}

// NewSubAgentRunnerAdapter creates a new adapter for sub-agent execution.
func NewSubAgentRunnerAdapter() *SubAgentRunnerAdapter {
	return &SubAgentRunnerAdapter{}
}

// SetCoordinator sets the coordinator reference.
// This is called after the coordinator is fully initialized.
func (a *SubAgentRunnerAdapter) SetCoordinator(c *coordinator) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.coordinator = c
}

// RunSubAgent executes a sub-agent with the given options.
func (a *SubAgentRunnerAdapter) RunSubAgent(ctx context.Context, opts plugin.SubAgentOptions) (string, error) {
	a.mu.RLock()
	c := a.coordinator
	a.mu.RUnlock()

	if c == nil {
		return "", errors.New("coordinator not initialized")
	}

	// Get session and message IDs from context.
	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return "", errors.New("session id missing from context")
	}

	messageID := tools.GetMessageFromContext(ctx)
	if messageID == "" {
		return "", errors.New("message id missing from context")
	}

	// Build tools list for the sub-agent.
	allowedTools := opts.AllowedTools
	if allowedTools == nil {
		// Inherit all tools from parent.
		allowedTools = c.getAllToolNames(ctx)
	}

	// Filter out disallowed tools.
	if len(opts.DisallowedTools) > 0 {
		filtered := make([]string, 0, len(allowedTools))
		for _, t := range allowedTools {
			if !slices.Contains(opts.DisallowedTools, t) {
				filtered = append(filtered, t)
			}
		}
		allowedTools = filtered
	}

	// Create a temporary agent config for the sub-agent.
	agentCfg := config.Agent{
		Name:         opts.Name,
		AllowedTools: allowedTools,
	}

	// Build the sub-agent with custom system prompt.
	subAgent, err := c.buildSubAgentWithPrompt(ctx, opts.SystemPrompt, agentCfg)
	if err != nil {
		return "", fmt.Errorf("build sub-agent: %w", err)
	}

	// Create sub-agent session.
	toolCallID := fmt.Sprintf("subagent-%s", opts.Name)
	agentToolSessionID := c.sessions.CreateAgentToolSessionID(messageID, toolCallID)
	session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, sessionID, "SubAgent: "+opts.Name)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	// Get model config.
	model := subAgent.Model()
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	providerCfg, ok := c.cfg.Providers.Get(model.ModelCfg.Provider)
	if !ok {
		return "", errors.New("model provider not configured")
	}

	// Run the sub-agent.
	result, err := subAgent.Run(ctx, SessionAgentCall{
		SessionID:        session.ID,
		Prompt:           opts.Prompt,
		MaxOutputTokens:  maxTokens,
		ProviderOptions:  getProviderOptions(model, providerCfg),
		Temperature:      model.ModelCfg.Temperature,
		TopP:             model.ModelCfg.TopP,
		TopK:             model.ModelCfg.TopK,
		FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
		PresencePenalty:  model.ModelCfg.PresencePenalty,
	})
	if err != nil {
		return "", fmt.Errorf("run sub-agent: %w", err)
	}

	// Update parent session with cost.
	updatedSession, err := c.sessions.Get(ctx, session.ID)
	if err != nil {
		return "", fmt.Errorf("get session: %w", err)
	}
	parentSession, err := c.sessions.Get(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("get parent session: %w", err)
	}

	parentSession.Cost += updatedSession.Cost
	_, err = c.sessions.Save(ctx, parentSession)
	if err != nil {
		return "", fmt.Errorf("save parent session: %w", err)
	}

	return result.Response.Content.Text(), nil
}

// getAllToolNames returns all available tool names.
func (c *coordinator) getAllToolNames(ctx context.Context) []string {
	// Get task agent config for tool list (it has the most tools).
	agentCfg, ok := c.cfg.Agents[config.AgentCoder]
	if !ok {
		return nil
	}
	return agentCfg.AllowedTools
}

// buildSubAgentWithPrompt builds a sub-agent with a custom system prompt.
func (c *coordinator) buildSubAgentWithPrompt(ctx context.Context, systemPrompt string, agent config.Agent) (SessionAgent, error) {
	large, small, err := c.buildAgentModels(ctx, true)
	if err != nil {
		return nil, err
	}

	largeProviderCfg, _ := c.cfg.Providers.Get(large.ModelCfg.Provider)
	result := NewSessionAgent(SessionAgentOptions{
		large,
		small,
		largeProviderCfg.SystemPromptPrefix,
		"",
		true, // isSubAgent
		c.cfg.Options.DisableAutoSummarize,
		c.permissions.SkipRequests(),
		c.sessions,
		c.messages,
		nil,
	})

	// Build tools for the sub-agent.
	subAgentTools, err := c.buildSubAgentTools(ctx, agent)
	if err != nil {
		return nil, err
	}

	// Set the custom system prompt directly.
	if systemPrompt != "" {
		// Use custom system prompt as-is. The plugin provides the full prompt.
		result.SetSystemPrompt(systemPrompt)
	} else {
		// Build default task prompt asynchronously.
		c.readyWg.Go(func() error {
			taskP, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
			if err != nil {
				return err
			}
			p, err := taskP.Build(ctx, large.Model.Provider(), large.Model.Model(), *c.cfg)
			if err != nil {
				return err
			}
			result.SetSystemPrompt(p)
			return nil
		})
	}

	result.SetTools(subAgentTools)

	return result, nil
}

// buildSubAgentTools builds the tool list for a sub-agent.
func (c *coordinator) buildSubAgentTools(ctx context.Context, agent config.Agent) ([]fantasy.AgentTool, error) {
	// Build all available tools.
	var allTools []fantasy.AgentTool

	// Get the model name for the agent.
	modelName := ""
	if modelCfg, ok := c.cfg.Models[agent.Model]; ok {
		if model := c.cfg.GetModel(modelCfg.Provider, modelCfg.Model); model != nil {
			modelName = model.Name
		}
	}

	// Add core tools.
	allTools = append(allTools,
		tools.NewBashTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Options.Attribution, modelName),
		tools.NewJobOutputTool(),
		tools.NewJobKillTool(),
		tools.NewDownloadTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewEditTool(c.lspClients, c.permissions, c.history, c.filetracker, c.cfg.WorkingDir()),
		tools.NewMultiEditTool(c.lspClients, c.permissions, c.history, c.filetracker, c.cfg.WorkingDir()),
		tools.NewFetchTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewGlobTool(c.cfg.WorkingDir()),
		tools.NewGrepTool(c.cfg.WorkingDir()),
		tools.NewLsTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Tools.Ls),
		tools.NewSourcegraphTool(nil),
		tools.NewTodosTool(c.sessions),
		tools.NewViewTool(c.lspClients, c.permissions, c.filetracker, c.cfg.WorkingDir(), c.cfg.Options.SkillsPaths...),
		tools.NewWriteTool(c.lspClients, c.permissions, c.history, c.filetracker, c.cfg.WorkingDir()),
	)

	if c.lspClients.Len() > 0 {
		allTools = append(allTools, tools.NewDiagnosticsTool(c.lspClients), tools.NewReferencesTool(c.lspClients), tools.NewLSPRestartTool(c.lspClients))
	}

	// Add plugin tools.
	for _, name := range plugin.RegisteredTools() {
		if c.pluginApp.IsPluginDisabled(name) {
			continue
		}
		factory, ok := plugin.GetToolFactory(name)
		if !ok {
			continue
		}
		tool, err := factory(ctx, c.pluginApp)
		if err != nil {
			continue
		}
		allTools = append(allTools, tool)
	}

	// Filter to allowed tools only.
	var filteredTools []fantasy.AgentTool
	for _, tool := range allTools {
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	// Add MCP tools.
	for _, tool := range tools.GetMCPTools(c.permissions, c.cfg.WorkingDir()) {
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	slices.SortFunc(filteredTools, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})

	return filteredTools, nil
}
