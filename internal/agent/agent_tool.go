package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"charm.land/fantasy"
	"golang.org/x/sync/errgroup"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/subagents"
)

//go:embed templates/agent_tool.md
var agentToolDescription string

// AgentParams is the input to the dispatcher agent tool, also decoded by the
// UI tool-call renderers. Inputs stored before subagent_type existed carry
// only a prompt and still decode.
type AgentParams struct {
	SubagentType string `json:"subagent_type,omitempty"`
	Prompt       string `json:"prompt"`
	Model        string `json:"model,omitempty"`
	Provider     string `json:"provider,omitempty"`
}

const (
	AgentToolName = "agent"

	// askUserForModel ends every dispatch error caused by a requested model,
	// so an unknown or ambiguous choice goes back to the user instead of
	// being guessed.
	askUserForModel = "Do not guess or fall back: ask the user which model/provider to use and wait for their answer."
)

// dispatcherTool implements fantasy.AgentTool with a dynamically-built schema.
type dispatcherTool struct {
	info         fantasy.ToolInfo
	dispatch     func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error)
	providerOpts fantasy.ProviderOptions
}

func (d *dispatcherTool) Info() fantasy.ToolInfo                          { return d.info }
func (d *dispatcherTool) ProviderOptions() fantasy.ProviderOptions        { return d.providerOpts }
func (d *dispatcherTool) SetProviderOptions(opts fantasy.ProviderOptions) { d.providerOpts = opts }
func (d *dispatcherTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var params AgentParams
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

// bypassPermissionsToolName is the permission tool name for confirming a
// project subagent's bypassPermissions, kept apart from AgentToolName.
const bypassPermissionsToolName = "agent_bypass_permissions"

// confirmBypassPermissions gates permissionMode: bypassPermissions behind an
// explicit user confirmation for every dispatch of a subagent that is not
// user-scoped. A repository can ship a subagent definition with a description
// crafted to get auto-dispatched, so repo-provided bypass must never
// auto-approve a whole child session without the user seeing it. Returns
// (zero, true) when dispatch may proceed and (denial response, false)
// otherwise. Yolo mode and auto-approved sessions are honored by the
// permission service. An allowlist entry or hook approval for the agent tool
// approves dispatching, not auto-approving the child session, so the prompt
// uses its own tool name and drops the call's hook approval.
func (c *coordinator) confirmBypassPermissions(ctx context.Context, sa *subagents.Subagent, sessionID, toolCallID string) (fantasy.ToolResponse, bool) {
	if sa.PermissionMode != subagents.PermissionModeBypassPermissions || subagents.InGlobalDir(sa.FilePath) {
		return fantasy.ToolResponse{}, true
	}
	granted, err := c.permissions.Request(permission.WithHookApproval(ctx, ""), permission.CreatePermissionRequest{
		SessionID:   sessionID,
		ToolCallID:  toolCallID,
		ToolName:    bypassPermissionsToolName,
		Description: fmt.Sprintf("Subagent %q is defined in this project and requests bypassPermissions: it would run with every tool call auto-approved.", sa.Name),
		Action:      sa.Name,
		Path:        sa.FilePath,
	})
	if err != nil || !granted {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("subagent %q requests bypassPermissions and the user did not approve it", sa.Name)), false
	}
	return fantasy.ToolResponse{}, true
}

// buildAgentDispatchInfo builds the ToolInfo for the agent dispatcher tool with
// a dynamic subagent_type enum derived from the currently active subagents.
// models supplies the selected large/small ids named in the model parameter's
// description as examples.
func buildAgentDispatchInfo(activeSubagents []*subagents.Subagent, models map[config.SelectedModelType]config.SelectedModel) fantasy.ToolInfo {
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

	modelDesc := `Model for this dispatch only, overriding the agent's default: "large", "small", or a model ID from a configured provider`
	if large, small := models[config.SelectedModelTypeLarge].Model, models[config.SelectedModelTypeSmall].Model; large != "" && small != "" {
		modelDesc += fmt.Sprintf(" (currently large is %q and small is %q)", large, small)
	}
	modelDesc += ". Set it ONLY when the user explicitly asks for a model; otherwise omit it. " +
		"If the dispatch fails because the model is unknown or ambiguous, do not guess another ID, " +
		"drop this parameter, or do the work yourself: show the user the options from the error and wait for their choice."

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
			"model": map[string]any{
				"type":        "string",
				"description": modelDesc,
			},
			"provider": map[string]any{
				"type":        "string",
				"description": "Provider for model. Set it only when the user names one, e.g. after choosing between providers that offer the same model ID.",
			},
		},
		Required: []string{"prompt"},
		Parallel: true,
	}
}

// agentTool builds the dispatcher tool for owner, whose tools cap every
// custom subagent it dispatches (plan-mode subagents stay read-only). The
// context parameter is retained for call-site symmetry with the other
// buildTools helpers; the task agent is built from the dispatch context.
func (c *coordinator) agentTool(_ context.Context, owner config.Agent) (fantasy.AgentTool, error) {
	taskCfg, ok := c.cfg.Config().Agents[config.AgentTask]
	if !ok {
		return nil, errors.New("task agent not configured")
	}
	// task.md.tpl never renders skills, so skip the discovery walk.
	taskPr, err := taskPrompt(
		prompt.WithWorkingDir(c.cfg.WorkingDir()),
		prompt.WithSuppressAvailableSkills(true),
	)
	if err != nil {
		return nil, err
	}
	// The task agent is built on first dispatch, not here. Two reasons it does
	// not go on c.readyWg: UpdateModels rebuilds this tool at the start of
	// every turn — after that turn's readyWg.Wait — so a readyWg-spawned build
	// could still be pending when a task dispatch runs (starting the agent
	// promptless/toolless), and a build failure would stick in readyWg, failing
	// every later turn.
	//
	// It is not built eagerly here either. buildAgent spawns a full skills
	// discovery walk plus an MCP-init wait, and nothing joins those goroutines
	// unless a task is actually dispatched — so eagerly building meant every
	// turn started a generation of work that the great majority of turns threw
	// away, with no backpressure across a burst of turns. Building on demand
	// makes an unused tool free and matches the subagent dispatch path below,
	// which also builds at dispatch time. The mutex serializes the concurrent
	// dispatches this tool allows (Parallel: true) onto one build, but unlike
	// sync.Once a failed build does not stick: the next dispatch retries.
	var (
		taskMu    sync.Mutex
		taskAgent SessionAgent
		taskBuilt bool
	)
	buildTaskAgent := func(ctx context.Context) (SessionAgent, error) {
		taskMu.Lock()
		defer taskMu.Unlock()
		if taskBuilt {
			return taskAgent, nil
		}
		var wg errgroup.Group
		agent, err := c.buildAgent(ctx, taskPr, taskCfg, true, subagentModel{}, &wg)
		if err == nil {
			err = wg.Wait()
		}
		if err != nil {
			// Leave taskBuilt unset so a transient build failure (a cancelled
			// dispatch context, a provider hiccup) is retried by the next
			// dispatch instead of sticking for the tool's whole lifetime.
			return nil, err
		}
		taskAgent = agent
		taskBuilt = true
		return taskAgent, nil
	}

	// The subagent_type enum is a snapshot taken when the tool is built, at
	// the start of each turn, so a Library reload shows up from the next
	// turn. Dispatch lookups use the live list (activeSubagentsList) so a
	// name removed mid-turn fails cleanly and a newly added one still
	// resolves — the enum is advisory only.
	info := buildAgentDispatchInfo(c.activeSubagentsList(), c.cfg.Config().Models)

	return &dispatcherTool{
		info: info,
		dispatch: func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}
			if params.Provider != "" && params.Model == "" {
				return fantasy.NewTextErrorResponse("provider requires model. " + askUserForModel), nil
			}
			buildFailed := func(what string, err error) (fantasy.ToolResponse, error) {
				msg := fmt.Sprintf("build %s: %v", what, err)
				if params.Model != "" {
					msg += ". " + askUserForModel
				}
				return fantasy.NewTextErrorResponse(msg), nil
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
				var taskAgent SessionAgent
				var err error
				if params.Model == "" {
					taskAgent, err = buildTaskAgent(ctx)
				} else {
					// A requested model gets its own agent per dispatch; the
					// model itself is memoized by resolveModelByID.
					var wg errgroup.Group
					taskAgent, err = c.buildAgent(ctx, taskPr, taskCfg, true, subagentModel{Model: params.Model, Provider: params.Provider}, &wg)
					if err == nil {
						err = wg.Wait()
					}
				}
				if err != nil {
					return buildFailed("task agent", err)
				}
				return c.runSubAgent(ctx, subAgentParams{
					Agent:          taskAgent,
					SessionID:      sessionID,
					AgentMessageID: agentMessageID,
					ToolCallID:     call.ID,
					Prompt:         params.Prompt,
					SessionTitle:   "New Agent Session",
					AgentName:      config.AgentTask,
					AgentColor:     subagents.AutoColor(config.AgentTask),
					AgentModel:     taskAgent.Model().ModelCfg.Model,
				})
			}

			sa := findSubagentByName(c.activeSubagentsList(), subagentType)
			if sa == nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("unknown subagent type: %q", subagentType)), nil
			}

			if resp, ok := c.confirmBypassPermissions(ctx, sa, sessionID, call.ID); !ok {
				return resp, nil
			}

			agentCfg := sa.ToConfigAgent(owner)
			// Config-driven setup failures (prompt build, model/provider that
			// passed discovery but fails at build) are surfaced as tool-error
			// responses so the parent agent can report them and continue; a
			// bare error would abort the whole turn.
			activeSkills := c.activeSkillsList()
			promptOpts := []prompt.Option{
				prompt.WithWorkingDir(c.cfg.WorkingDir()),
				// Reuse the skills the coordinator already holds instead of
				// letting prompt.Build re-walk every configured skills path.
				// This tool is Parallel, so N concurrent dispatches would
				// otherwise each pay a full recursive walk before their first
				// token.
				prompt.WithAvailableSkillsXML(skills.ToPromptXML(activeSkills)),
			}
			// Skills are activated by viewing their SKILL.md; without the
			// view tool the list is only instructions it cannot follow.
			if !slices.Contains(agentCfg.AllowedTools, tools.ViewToolName) {
				promptOpts = append(promptOpts, prompt.WithSuppressAvailableSkills(true))
			}
			subPr, err := subagentPrompt(sa, activeSkills, promptOpts...)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("build subagent prompt %q: %v", sa.Name, err)), nil
			}
			// Build on a local group and wait before running: the agent must
			// not start promptless/toolless, and a build failure must land
			// here as a tool error rather than in the coordinator-wide
			// readyWg, whose sticky error would fail every subsequent turn.
			sm := subagentModel{Effort: sa.Effort, Model: sa.Model, Provider: sa.Provider}
			if params.Model != "" {
				sm.Model, sm.Provider = params.Model, params.Provider
			}
			var buildWg errgroup.Group
			agent, err := c.buildAgent(ctx, subPr, agentCfg, true, sm, &buildWg)
			if err == nil {
				err = buildWg.Wait()
			}
			if err != nil {
				return buildFailed(fmt.Sprintf("subagent %q", sa.Name), err)
			}

			return c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   sa.Name + " Agent Session",
				SessionSetup:   c.subagentSessionSetup(sa),
				AgentName:      sa.Name,
				AgentColor:     sa.ResolvedColor(),
				AgentModel:     agent.Model().ModelCfg.Model,
			})
		},
	}, nil
}
