package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagents"
)

//go:embed templates/agent_tool.md
var agentToolDescription string

// AgentParams is the shape consumed by UI tool-call renderers when displaying
// historical agent tool invocations. New tool-call inputs decode with
// AgentDispatchParams; AgentParams stays wire-compatible so older inputs still
// decode cleanly.
type AgentParams struct {
	SubagentType string `json:"subagent_type,omitempty"`
	Prompt       string `json:"prompt" description:"The task for the agent to perform"`
}

// AgentDispatchParams is the input to the dispatcher agent tool.
type AgentDispatchParams struct {
	SubagentType string `json:"subagent_type,omitempty"`
	Prompt       string `json:"prompt"`
}

const (
	AgentToolName = "agent"
)

// dispatcherTool implements fantasy.AgentTool with a dynamically-built schema.
type dispatcherTool struct {
	info         fantasy.ToolInfo
	dispatch     func(ctx context.Context, params AgentDispatchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error)
	providerOpts fantasy.ProviderOptions
}

func (d *dispatcherTool) Info() fantasy.ToolInfo                          { return d.info }
func (d *dispatcherTool) ProviderOptions() fantasy.ProviderOptions        { return d.providerOpts }
func (d *dispatcherTool) SetProviderOptions(opts fantasy.ProviderOptions) { d.providerOpts = opts }
func (d *dispatcherTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var params AgentDispatchParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return fantasy.NewTextErrorResponse("invalid parameters: " + err.Error()), nil
	}
	return d.dispatch(ctx, params, call)
}

// findSubagentByName returns the active subagent with the given name, or nil
// when none matches.
func findSubagentByName(active []*subagents.Subagent, name string) *subagents.Subagent {
	for _, sa := range active {
		if sa.Name == name {
			return sa
		}
	}
	return nil
}

// subagentSessionSetup returns a SessionSetup callback that applies the
// subagent's permission mode to the freshly-created sub-session. Returns
// nil when no setup is needed.
func (c *coordinator) subagentSessionSetup(sa *subagents.Subagent) func(sessionID string) {
	if sa.PermissionMode != subagents.PermissionModeBypassPermissions {
		return nil
	}
	return func(sessionID string) {
		c.permissions.AutoApproveSession(sessionID)
	}
}

// buildAgentDispatchInfo builds the ToolInfo for the agent dispatcher tool with
// a dynamic subagent_type enum derived from the currently active subagents.
func buildAgentDispatchInfo(activeSubagents []*subagents.Subagent) fantasy.ToolInfo {
	enumValues := []string{"task"}
	for _, sa := range activeSubagents {
		enumValues = append(enumValues, sa.Name)
	}

	typeDesc := `The type of agent to use. Use "task" for general search and research tasks.`
	if len(activeSubagents) > 0 {
		lines := make([]string, 0, len(activeSubagents))
		for _, sa := range activeSubagents {
			lines = append(lines, fmt.Sprintf("- %s: %s", sa.Name, sa.Description))
		}
		typeDesc += "\n\nAvailable specialized agents:\n" + strings.Join(lines, "\n")
	}

	return fantasy.ToolInfo{
		Name:        AgentToolName,
		Description: agentToolDescription,
		Parameters: map[string]any{
			"subagent_type": map[string]any{
				"type":        "string",
				"enum":        enumValues,
				"description": typeDesc,
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "The task for the agent to perform",
			},
		},
		Required: []string{"prompt"},
		Parallel: true,
	}
}

func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
	taskCfg, ok := c.cfg.Config().Agents[config.AgentTask]
	if !ok {
		return nil, errors.New("task agent not configured")
	}
	coderCfg, ok := c.cfg.Config().Agents[config.AgentCoder]
	if !ok {
		return nil, errors.New("coder agent not configured")
	}
	taskPr, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}
	taskAgent, err := c.buildAgent(ctx, taskPr, taskCfg, true)
	if err != nil {
		return nil, err
	}

	info := buildAgentDispatchInfo(c.activeSubagents)

	return &dispatcherTool{
		info: info,
		dispatch: func(ctx context.Context, params AgentDispatchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
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

			subagentType := params.SubagentType
			if subagentType == "" || subagentType == config.AgentTask {
				return c.runSubAgent(ctx, subAgentParams{
					Agent:          taskAgent,
					SessionID:      sessionID,
					AgentMessageID: agentMessageID,
					ToolCallID:     call.ID,
					Prompt:         params.Prompt,
					SessionTitle:   "New Agent Session",
				})
			}

			sa := findSubagentByName(c.activeSubagents, subagentType)
			if sa == nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("unknown subagent type: %q", subagentType)), nil
			}

			agentCfg := sa.ToConfigAgent(coderCfg)
			subPr, err := subagentPrompt(sa, c.activeSkills, prompt.WithWorkingDir(c.cfg.WorkingDir()))
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("build subagent prompt %q: %w", sa.Name, err)
			}
			agent, err := c.buildAgent(ctx, subPr, agentCfg, true)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("build subagent %q: %w", sa.Name, err)
			}

			return c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   sa.Name + " Agent Session",
				SessionSetup:   c.subagentSessionSetup(sa),
			})
		},
	}, nil
}
