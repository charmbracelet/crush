package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/subagent"
)

//go:embed templates/subagent_tool.md
var subagentToolDescription []byte

// SubagentParams are the parameters for the subagent tool.
type SubagentParams struct {
	Prompt   string `json:"prompt" description:"The task for the agent to perform"`
	Subagent string `json:"subagent,omitempty" description:"Optional: name of specific subagent to use (e.g., 'code-reviewer')"`
}

const (
	SubagentToolName = "agent"
)

// subagentTool creates a tool for invoking user-defined subagents.
func (c *coordinator) subagentTool(ctx context.Context) (fantasy.AgentTool, error) {
	// Discover available subagents.
	homeDir, _ := os.UserHomeDir()
	discoveryPaths := subagent.DefaultDiscoveryPaths(homeDir, c.cfg.WorkingDir())
	subagents, err := subagent.Discover(discoveryPaths)
	if err != nil {
		slog.Warn("failed to discover subagents", "error", err)
		subagents = []*subagent.Subagent{}
	}

	// Build description with available subagents.
	description := buildSubagentDescription(subagents)

	return fantasy.NewParallelAgentTool(
		SubagentToolName,
		description,
		func(ctx context.Context, params SubagentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
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

			// Find the specified subagent or use default task agent.
			var selectedSubagent *subagent.Subagent
			if params.Subagent != "" {
				selectedSubagent = subagent.FindByName(subagents, params.Subagent)
				if selectedSubagent == nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("subagent '%s' not found", params.Subagent)), nil
				}
			}

			// Create the agent for this invocation.
			agent, err := c.buildSubagentAgent(ctx, selectedSubagent)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error building subagent: %w", err)
			}

			agentToolSessionID := c.sessions.CreateAgentToolSessionID(agentMessageID, call.ID)
			session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, sessionID, "Subagent Session")
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
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
				return fantasy.NewTextErrorResponse("error generating response"), nil
			}

			updatedSession, err := c.sessions.Get(ctx, session.ID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error getting session: %s", err)
			}
			parentSession, err := c.sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error getting parent session: %s", err)
			}

			parentSession.Cost += updatedSession.Cost

			_, err = c.sessions.Save(ctx, parentSession)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error saving parent session: %s", err)
			}

			return fantasy.NewTextResponse(result.Response.Content.Text()), nil
		}), nil
}

// buildSubagentAgent creates an agent for the given subagent definition.
func (c *coordinator) buildSubagentAgent(ctx context.Context, sa *subagent.Subagent) (SessionAgent, error) {
	// Use default task agent configuration if no subagent specified.
	if sa == nil {
		agentCfg, ok := c.cfg.Agents[config.AgentTask]
		if !ok {
			return nil, errors.New("task agent not configured")
		}
		p, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
		if err != nil {
			return nil, err
		}
		return c.buildAgent(ctx, p, agentCfg, true)
	}

	// Create a permission service for this subagent.
	subagentPermissions := c.createSubagentPermissions(sa)

	// Build agent config from subagent definition.
	agentCfg := config.Agent{
		Name:         sa.Name,
		Description:  sa.Description,
		Model:        config.SelectedModelTypeLarge,
		AllowedTools: sa.Tools,
	}

	// If no tools specified, use all tools.
	if len(agentCfg.AllowedTools) == 0 {
		agentCfg.AllowedTools = config.AllToolNames()
	}

	// Build the prompt.
	p, err := subagentPrompt(sa, prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	// Build models.
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
		subagentPermissions.SkipRequests(),
		c.sessions,
		c.messages,
		nil,
	})

	// Build system prompt.
	systemPrompt, err := p.Build(ctx, large.Model.Provider(), large.Model.Model(), *c.cfg)
	if err != nil {
		return nil, err
	}
	result.SetSystemPrompt(systemPrompt)

	// Build tools with subagent-specific permissions.
	tools, err := c.buildSubagentTools(ctx, agentCfg, subagentPermissions)
	if err != nil {
		return nil, err
	}
	result.SetTools(tools)

	return result, nil
}

// createSubagentPermissions creates a permission service for a subagent.
// If yolo_mode is true, all requests are auto-approved.
// Otherwise, allowed_tools are auto-approved and others bubble up.
func (c *coordinator) createSubagentPermissions(sa *subagent.Subagent) permission.Service {
	if sa.YoloMode {
		// Auto-approve everything.
		return permission.NewPermissionService(c.cfg.WorkingDir(), true, nil)
	}

	// Use subagent's allowed_tools, falling back to parent's allowed_tools.
	allowedTools := sa.AllowedTools
	if len(allowedTools) == 0 && c.cfg.Permissions != nil {
		allowedTools = c.cfg.Permissions.AllowedTools
	}

	// Create a permission service that delegates non-allowed tools to the parent.
	// For now, we create a separate service that shares the allowed tools list.
	// Permission requests for tools not in allowed_tools will prompt the user.
	return permission.NewPermissionService(c.cfg.WorkingDir(), c.permissions.SkipRequests(), allowedTools)
}

// buildSubagentTools builds tools for a subagent with custom permissions.
func (c *coordinator) buildSubagentTools(ctx context.Context, agent config.Agent, permissions permission.Service) ([]fantasy.AgentTool, error) {
	var allTools []fantasy.AgentTool

	// Get the model name for the agent.
	modelName := ""
	if modelCfg, ok := c.cfg.Models[agent.Model]; ok {
		if model := c.cfg.GetModel(modelCfg.Provider, modelCfg.Model); model != nil {
			modelName = model.Name
		}
	}

	allTools = append(allTools,
		tools.NewBashTool(permissions, c.cfg.WorkingDir(), c.cfg.Options.Attribution, modelName),
		tools.NewJobOutputTool(),
		tools.NewJobKillTool(),
		tools.NewDownloadTool(permissions, c.cfg.WorkingDir(), nil),
		tools.NewEditTool(c.lspClients, permissions, c.history, c.cfg.WorkingDir()),
		tools.NewMultiEditTool(c.lspClients, permissions, c.history, c.cfg.WorkingDir()),
		tools.NewFetchTool(permissions, c.cfg.WorkingDir(), nil),
		tools.NewGlobTool(c.cfg.WorkingDir()),
		tools.NewGrepTool(c.cfg.WorkingDir()),
		tools.NewLsTool(permissions, c.cfg.WorkingDir(), c.cfg.Tools.Ls),
		tools.NewSourcegraphTool(nil),
		tools.NewTodosTool(c.sessions),
		tools.NewViewTool(c.lspClients, permissions, c.cfg.WorkingDir(), c.cfg.Options.SkillsPaths...),
		tools.NewWriteTool(c.lspClients, permissions, c.history, c.cfg.WorkingDir()),
	)

	if len(c.cfg.LSP) > 0 {
		allTools = append(allTools, tools.NewDiagnosticsTool(c.lspClients), tools.NewReferencesTool(c.lspClients))
	}

	var filteredTools []fantasy.AgentTool
	for _, tool := range allTools {
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	// Add MCP tools with same filtering logic.
	for _, tool := range tools.GetMCPTools(permissions, c.cfg.WorkingDir()) {
		if agent.AllowedMCP == nil {
			filteredTools = append(filteredTools, tool)
			continue
		}
		if len(agent.AllowedMCP) == 0 {
			break
		}

		for mcp, mcpTools := range agent.AllowedMCP {
			if mcp != tool.MCP() {
				continue
			}
			if len(mcpTools) == 0 || slices.Contains(mcpTools, tool.MCPToolName()) {
				filteredTools = append(filteredTools, tool)
			}
		}
	}

	slices.SortFunc(filteredTools, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})

	return filteredTools, nil
}

// subagentPrompt creates a prompt for a user-defined subagent.
func subagentPrompt(sa *subagent.Subagent, opts ...prompt.Option) (*prompt.Prompt, error) {
	return prompt.NewPrompt(sa.Name, sa.Prompt, opts...)
}

// buildSubagentDescription creates the tool description including available subagents.
func buildSubagentDescription(subagents []*subagent.Subagent) string {
	var sb strings.Builder
	sb.WriteString(string(subagentToolDescription))

	if len(subagents) > 0 {
		sb.WriteString("\n\n<available_subagents>\n")
		sb.WriteString("You can specify a subagent by name using the 'subagent' parameter:\n")
		for _, sa := range subagents {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", sa.Name, firstLine(sa.Description)))
		}
		sb.WriteString("</available_subagents>\n")
	}

	return sb.String()
}

// firstLine returns the first line of a string.
func firstLine(s string) string {
	idx := strings.IndexAny(s, "\n\r")
	if idx == -1 {
		return s
	}
	return s[:idx]
}
