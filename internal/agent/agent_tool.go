package agent

import (
	"context"
	_ "embed"
	"errors"
	"slices"

	"charm.land/fantasy"
	"golang.org/x/sync/errgroup"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/agent_tool.md
var agentToolDescription string

type AgentParams struct {
	Prompt string `json:"prompt" description:"The task for the agent to perform"`
	Access string `json:"access,omitempty" description:"\"read\" (default) for a sub-agent that can only inspect the codebase, or \"write\" for one that can also edit files"`
}

const (
	AgentToolName = "agent"

	accessRead  = "read"
	accessWrite = "write"
)

// writeAgentConfig derives the read-write variant of the task agent: the same
// agent, given the coder's tools. The agent tool itself is withheld so
// sub-agents cannot fan out again.
func writeAgentConfig(cfg *config.Config) config.Agent {
	task := cfg.Agents[config.AgentTask]
	coder, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return task
	}
	task.AllowedTools = slices.DeleteFunc(slices.Clone(coder.AllowedTools), func(name string) bool {
		return name == AgentToolName ||
			name == AgentDispatchToolName ||
			name == AgentSendToolName ||
			name == AgentStatusToolName ||
			name == AgentStopToolName
	})
	return task
}

// readOnlyParent reports whether a parent agent may only delegate to
// read-only sub-agents. Plan mode promises it cannot change files, and that
// promise has to cover the work it hands off as well as the work it does
// itself.
func readOnlyParent(parentAgentID string) bool {
	return parentAgentID == config.AgentPlan
}

// resolveSubAgent picks the sub-agent variant for a requested access level,
// returning a message for the model when there is no such variant.
func resolveSubAgent(agents map[string]SessionAgent, access string, readOnly bool) (SessionAgent, string) {
	if access == "" {
		access = accessRead
	}
	if agent, ok := agents[access]; ok {
		return agent, ""
	}
	if readOnly && access == accessWrite {
		return nil, `plan mode sub-agents are read-only; omit access or pass "read"`
	}
	return nil, `access must be "read" or "write"`
}

// taskAgents builds the variants of the task sub-agent. Both the synchronous
// agent tool and the detached dispatch tool draw from these. A read-only
// parent gets only the read variant, so the write one is never built rather
// than built and withheld.
func (c *coordinator) taskAgents(ctx context.Context, readOnly bool) (map[string]SessionAgent, error) {
	if _, ok := c.cfg.Config().Agents[config.AgentTask]; !ok {
		return nil, errors.New("task agent not configured")
	}
	accesses := []string{accessRead, accessWrite}
	if readOnly {
		accesses = accesses[:1]
	}

	// The variants share a readiness group of their own and are fully set
	// up before this returns. This runs on every turn, concurrently with
	// other runs waiting on the coordinator's group, so joining that group
	// would race its Wait.
	var ready errgroup.Group
	agents := map[string]SessionAgent{}
	for _, access := range accesses {
		agentCfg, canWrite := c.taskAgentConfig(access)
		agent, err := c.buildTaskAgent(ctx, &ready, agentCfg, canWrite, nil)
		if err != nil {
			return nil, err
		}
		agents[access] = agent
	}
	if err := ready.Wait(); err != nil {
		return nil, err
	}
	return agents, nil
}

// taskAgentConfig returns the task agent's config for an access level, and
// whether it may write.
func (c *coordinator) taskAgentConfig(access string) (config.Agent, bool) {
	if access == accessWrite {
		return writeAgentConfig(c.cfg.Config()), true
	}
	return c.cfg.Config().Agents[config.AgentTask], false
}

// buildTaskAgent schedules one variant of the task sub-agent into ready; the
// caller waits on the group, so several variants can build concurrently.
// modelsFn, when set, replaces the shared sub-agent model set; a dispatch
// with a model override uses it to run the variant on the overridden model.
func (c *coordinator) buildTaskAgent(ctx context.Context, ready *errgroup.Group, agentCfg config.Agent, canWrite bool, modelsFn func() ModelSet) (SessionAgent, error) {
	pr, err := taskPrompt(
		prompt.WithWorkingDir(c.cfg.WorkingDir()),
		prompt.WithCanWrite(canWrite),
	)
	if err != nil {
		return nil, err
	}
	return c.buildAgentWithModels(ctx, ready, pr, agentCfg, true, modelsFn)
}

func (c *coordinator) agentTool(ctx context.Context, parentAgentID string) (fantasy.AgentTool, error) {
	readOnly := readOnlyParent(parentAgentID)
	agents, err := c.taskAgents(ctx, readOnly)
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

			agent, refusal := resolveSubAgent(agents, params.Access, readOnly)
			if refusal != "" {
				return fantasy.NewTextErrorResponse(refusal), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			return c.runSubAgentMaybeBackground(ctx, AgentToolName, params.Access, subAgentParams{
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
