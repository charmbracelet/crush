package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"

	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/agent_tool.md
var agentToolDescription []byte

type AgentTaskParams struct {
	ID           string   `json:"id" description:"The unique task identifier used for dependency references"`
	Description  string   `json:"description,omitempty" description:"A short title for the delegated task"`
	Prompt       string   `json:"prompt" description:"The task for the agent to perform"`
	SubagentType string   `json:"subagent_type,omitempty" description:"The subagent type to use: general, explore, or a configured subagent name"`
	DependsOn    []string `json:"depends_on,omitempty" description:"Task IDs that must complete successfully before this task runs"`
}

type AgentParams struct {
	Description  string            `json:"description,omitempty" description:"A short title for the delegated task"`
	Prompt       string            `json:"prompt,omitempty" description:"The task for the agent to perform"`
	SubagentType string            `json:"subagent_type,omitempty" description:"The subagent type to use: general, explore, or a configured subagent name"`
	Tasks        []AgentTaskParams `json:"tasks,omitempty" description:"Optional task graph with dependency-aware delegation"`
}

const (
	AgentToolName = "agent"
)

func (c *coordinator) agentTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		AgentToolName,
		c.buildAgentToolDescription(),
		func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			// Unified path: construct task graph and delegate to runTaskGraph
			var tasks []taskGraphTask
			if len(params.Tasks) > 0 {
				for _, task := range params.Tasks {
					if strings.TrimSpace(task.ID) == "" {
						return fantasy.NewTextErrorResponse("task id is required"), nil
					}
					if strings.TrimSpace(task.Prompt) == "" {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("task %q prompt is required", task.ID)), nil
					}
					tasks = append(tasks, taskGraphTask{
						ID:           strings.TrimSpace(task.ID),
						Description:  task.Description,
						Prompt:       task.Prompt,
						SubagentType: task.SubagentType,
						DependsOn:    task.DependsOn,
					})
				}
			} else {
				// Single-task case: convert to single-node task graph
				if params.Prompt == "" {
					return fantasy.NewTextErrorResponse("prompt is required"), nil
				}
				description := strings.TrimSpace(params.Description)
				if description == "" {
					// Use subagent type to determine default description
					subagentType := config.CanonicalSubagentID(strings.TrimSpace(params.SubagentType))
					description = defaultSubagentDescription(subagentType, params.Prompt)
				}
				tasks = []taskGraphTask{{
					ID:           "task",
					Description:  description,
					Prompt:       params.Prompt,
					SubagentType: params.SubagentType,
				}}
			}

			return c.runTaskGraph(ctx, taskGraphParams{
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Tasks:          tasks,
			})
		}), nil
}

func (c *coordinator) buildAgentToolDescription() string {
	subagents := make([]config.Agent, 0)
	seen := make(map[string]struct{})
	for _, agentCfg := range c.cfg.Config().Agents {
		if config.NormalizeAgentMode(agentCfg.Mode) == config.AgentModePrimary {
			continue
		}
		canonicalID := config.CanonicalSubagentID(agentCfg.ID)
		if _, ok := seen[canonicalID]; ok {
			continue
		}
		seen[canonicalID] = struct{}{}
		subagents = append(subagents, agentCfg)
	}
	slices.SortFunc(subagents, func(a, b config.Agent) int {
		return strings.Compare(a.ID, b.ID)
	})

	entries := make([]string, 0, len(subagents))
	for _, agentCfg := range subagents {
		entries = append(entries, fmt.Sprintf("- %s: %s", config.CanonicalSubagentID(agentCfg.ID), agentCfg.Description))
	}

	return strings.ReplaceAll(string(agentToolDescription), "{agents}", strings.Join(entries, "\n"))
}

func (c *coordinator) buildSubAgentForType(ctx context.Context, requestedType string) (SessionAgent, config.Agent, error) {
	if c.subAgentFactory != nil {
		return c.subAgentFactory(ctx, requestedType)
	}

	agentCfg, err := c.subagentConfig(requestedType)
	if err != nil {
		return nil, config.Agent{}, err
	}

	promptBuilder, err := promptForAgent(agentCfg, true, prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, config.Agent{}, err
	}

	subAgent, err := c.buildAgent(ctx, promptBuilder, agentCfg, true)
	if err != nil {
		return nil, config.Agent{}, err
	}

	return subAgent, agentCfg, nil
}

func (c *coordinator) subagentConfig(requestedType string) (config.Agent, error) {
	subagentType := config.CanonicalSubagentID(strings.TrimSpace(requestedType))
	agentCfg, ok := c.cfg.Config().Agents[subagentType]
	if !ok {
		return config.Agent{}, fmt.Errorf("unknown subagent type: %s", subagentType)
	}
	if config.NormalizeAgentMode(agentCfg.Mode) == config.AgentModePrimary {
		return config.Agent{}, fmt.Errorf("agent %s is not available as a subagent", agentCfg.ID)
	}
	return agentCfg, nil
}

func defaultSubagentDescription(subagentType, prompt string) string {
	title := strings.TrimSpace(prompt)
	if title == "" {
		return titleCase(subagentType) + " task"
	}
	words := strings.Fields(title)
	if len(words) > 6 {
		words = words[:6]
	}
	return strings.Join(words, " ")
}

func formatSubagentSessionTitle(description, subagentType string) string {
	if description == "" {
		description = titleCase(subagentType) + " task"
	}
	return fmt.Sprintf("%s (@%s subagent)", description, subagentType)
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
