package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
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
	Access string `json:"access,omitempty" description:"\"read\" (default) for a subagent that can only inspect the codebase, or \"write\" for one that can also edit files"`
}

const (
	AgentToolName = "agent"

	accessRead  = "read"
	accessWrite = "write"
)

// narrowAgentTools returns agentCfg with its toolset cut down to names.
//
// The list only ever narrows: naming a tool the agent does not already have is
// an error rather than a grant, because a dispatch must not be able to widen
// what the caller could do. A nil list keeps the toolset untouched; an empty
// one removes every tool but the always-present ones effectiveToolNames keeps
// for a subagent.
func narrowAgentTools(agentCfg config.Agent, names []string) (config.Agent, error) {
	if names == nil {
		return agentCfg, nil
	}
	narrowed := slices.Clone(names)
	for _, name := range narrowed {
		if !slices.Contains(agentCfg.AllowedTools, name) {
			return config.Agent{}, fmt.Errorf(
				"tool %q is not available to this subagent; it may only use tools the caller already has",
				name,
			)
		}
	}
	agentCfg.AllowedTools = narrowed
	return agentCfg, nil
}

// writeAgentConfig derives the read-write variant of the task agent: the same
// agent, given the coder's tools. Subagents never get the delegation tools;
// effectiveToolNames strips them.
func writeAgentConfig(cfg *config.Config) config.Agent {
	task := cfg.Agents[config.AgentTask]
	if coder, ok := cfg.Agents[config.AgentCoder]; ok {
		task.AllowedTools = slices.Clone(coder.AllowedTools)
	}
	return task
}

// accessOrRead defaults an unset access level to read-only.
func accessOrRead(access string) string {
	if access == "" {
		return accessRead
	}
	return access
}

// readOnlyParent reports whether a parent agent may only delegate to
// read-only subagents. Plan mode promises it cannot change files, and that
// promise has to cover the work it hands off as well as the work it does
// itself.
func readOnlyParent(parentAgentID string) bool {
	return parentAgentID == config.AgentPlan
}

// resolveSubAgent picks the subagent variant for a requested access level,
// returning a message for the model when there is no such variant.
func resolveSubAgent(agents map[string]SessionAgent, access string, readOnly bool) (SessionAgent, string) {
	access = accessOrRead(access)
	if agent, ok := agents[access]; ok {
		return agent, ""
	}
	if readOnly && access == accessWrite {
		return nil, `plan mode subagents are read-only; omit access or pass "read"`
	}
	return nil, `access must be "read" or "write"`
}

// taskAgents builds the variants of the task subagent. Both the synchronous
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

// narrowTaskAgent builds a task subagent whose toolset is cut down to names.
//
// The shared read/write variants are built once per turn, so a per-call
// restriction cannot reuse them; this builds a dedicated agent instead. The
// list only narrows, so a dispatch can never hand a subagent a tool the caller
// does not already have.
func (c *coordinator) narrowTaskAgent(ctx context.Context, access string, names []string) (SessionAgent, error) {
	agentCfg, canWrite := c.taskAgentConfig(access)
	narrowed, err := narrowAgentTools(agentCfg, names)
	if err != nil {
		return nil, err
	}
	agent, err := c.buildNarrowedTaskAgent(ctx, narrowed, canWrite)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

// buildNarrowedTaskAgent builds a task subagent from an already-narrowed
// config. Kept separate so the decision about which tools an agent gets can be
// tested without a live coordinator, which building one requires.
func (c *coordinator) buildNarrowedTaskAgent(ctx context.Context, agentCfg config.Agent, canWrite bool) (SessionAgent, error) {
	var ready errgroup.Group
	agent, err := c.buildTaskAgent(ctx, &ready, agentCfg, canWrite, nil)
	if err != nil {
		return nil, err
	}
	if err := ready.Wait(); err != nil {
		return nil, err
	}
	return agent, nil
}

// buildTaskAgent schedules one variant of the task subagent into ready; the
// caller waits on the group, so several variants can build concurrently.
// modelsFn, when set, replaces the shared subagent model set; a dispatch
// with a model override uses it to run the variant on the overridden model.
func (c *coordinator) buildTaskAgent(ctx context.Context, ready *errgroup.Group, agentCfg config.Agent, canWrite bool, modelsFn func() ModelSet) (SessionAgent, error) {
	pr, err := c.newPrompt("task",
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
