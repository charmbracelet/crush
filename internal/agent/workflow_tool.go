package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/agent/workflow"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
)

//go:embed templates/workflow_tool.md
var workflowToolDescription string

// WorkflowToolName is the name of the workflow tool.
const WorkflowToolName = "workflow"

// Sub-agent profile names accepted by the Lua API. These must stay in sync
// with workflow.ValidAgentProfiles, which is what validates script input.
const (
	workflowProfileTask  = "task"
	workflowProfileCoder = "coder"
)

// WorkflowParams is the parameters for the workflow tool.
type WorkflowParams struct {
	Description string `json:"description" description:"One-line summary of what this workflow does; shown in the permission prompt"`
	Script      string `json:"script" description:"Lua orchestration script; see tool description for the API"`
}

// workflowAgentKey identifies one sub-agent variant: a profile ("task" or
// "coder") paired with the model a spawn asked for ("large", "small", or ""
// meaning inherit whatever the profile's config selects).
type workflowAgentKey struct {
	profile string
	model   string
}

// workflowSpawnKey maps one spawn's options to the sub-agent variant it needs.
// An empty profile defaults to "task"; an empty model inherits whatever that
// profile's config selects. Both are documented Lua API defaults, and the engine
// has already rejected values outside workflow.ValidAgentProfiles/ValidModels.
func workflowSpawnKey(opts workflow.SpawnOpts) workflowAgentKey {
	profile := opts.Agent
	if profile == "" {
		profile = workflowProfileTask
	}
	return workflowAgentKey{profile: profile, model: opts.Model}
}

// workflowAgents builds sub-agents on first use and caches them, so every
// spawn of a given kind shares one agent.
//
// Lazily rather than up front: buildTools runs on every prompt submit
// (coordinator.go), so eagerly constructing all four profile/model
// combinations would add provider setup to the latency of each message.
// Scripts that never ask for the small model pay exactly what they did before.
type workflowAgents struct {
	build func(key workflowAgentKey) (SessionAgent, error)

	mu    sync.Mutex
	cache map[workflowAgentKey]SessionAgent
}

// get returns the agent for key, building it if this is its first use.
// The build call deliberately happens outside the lock: agents are only ever
// added, so a concurrent duplicate build is wasteful but harmless, whereas
// holding a mutex across agent construction would serialise every spawn in a
// parallel batch behind the first one.
func (w *workflowAgents) get(key workflowAgentKey) (SessionAgent, error) {
	w.mu.Lock()
	cached, ok := w.cache[key]
	w.mu.Unlock()
	if ok {
		return cached, nil
	}

	built, err := w.build(key)
	if err != nil {
		return nil, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	// Another spawn may have won the race; keep whichever landed first so all
	// spawns of this kind share one agent.
	if cached, ok := w.cache[key]; ok {
		return cached, nil
	}
	w.cache[key] = built
	return built, nil
}

// stripWorkflowTool returns a copy of the agent config with the
// "workflow" tool removed from AllowedTools. This prevents recursive
// fan-out: a subagent must never invoke the workflow tool itself.
func stripWorkflowTool(agent config.Agent) config.Agent {
	agent.AllowedTools = slices.DeleteFunc(
		slices.Clone(agent.AllowedTools),
		func(s string) bool { return s == WorkflowToolName },
	)
	return agent
}

// newWorkflowAgents prepares the sub-agent factory for one workflow tool: both
// profile configs with the workflow tool stripped, both system prompts, and the
// lazy per-(profile, model) cache that spawn resolves against.
func (c *coordinator) newWorkflowAgents(ctx context.Context) (*workflowAgents, error) {
	// Build agents for both profiles, stripping the workflow tool
	// from each to prevent recursive fan-out (safety 3a).
	taskCfg, ok := c.cfg.Config().Agents[config.AgentTask]
	if !ok {
		return nil, errors.New("task agent configuration not found")
	}
	taskCfg = stripWorkflowTool(taskCfg)

	coderCfg, ok := c.cfg.Config().Agents[config.AgentCoder]
	if !ok {
		return nil, errors.New("coder agent configuration not found")
	}
	coderCfg = stripWorkflowTool(coderCfg)

	taskPrmpt, err := taskPrompt(
		prompt.WithWorkingDir(c.cfg.WorkingDir()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build task prompt: %w", err)
	}

	coderPrmpt, err := coderPrompt(
		prompt.WithWorkingDir(c.cfg.WorkingDir()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build coder prompt: %w", err)
	}

	agents := &workflowAgents{
		cache: make(map[workflowAgentKey]SessionAgent),
		build: func(key workflowAgentKey) (SessionAgent, error) {
			cfg, prmpt := taskCfg, taskPrmpt
			if key.profile == workflowProfileCoder {
				cfg, prmpt = coderCfg, coderPrmpt
			}

			// An empty model leaves the profile's configured selection alone,
			// which is what the Lua API documents as the default. The engine
			// has already rejected anything outside workflow.ValidModels.
			switch key.model {
			case "large":
				cfg.Model = config.SelectedModelTypeLarge
			case "small":
				cfg.Model = config.SelectedModelTypeSmall
			}

			agent, err := c.buildAgent(ctx, prmpt, cfg, true)
			if err != nil {
				return nil, fmt.Errorf("failed to build %s sub-agent: %w", key.profile, err)
			}
			return agent, nil
		},
	}

	return agents, nil
}

func (c *coordinator) workflowTool(ctx context.Context) (fantasy.AgentTool, error) {
	agents, err := c.newWorkflowAgents(ctx)
	if err != nil {
		return nil, err
	}

	return fantasy.NewAgentTool(
		WorkflowToolName,
		workflowToolDescription,
		func(ctx context.Context, params WorkflowParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Description == "" {
				return fantasy.NewTextErrorResponse("Description cannot be empty"), nil
			}
			if params.Script == "" {
				return fantasy.NewTextErrorResponse("Script cannot be empty"), nil
			}
			if len(params.Script) > workflow.MaxScriptBytes {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Script length exceeds maximum of %d bytes", workflow.MaxScriptBytes)), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session context missing")
			}
			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("message context missing")
			}

			// Safety 3c: state the capability this approval grants.
			// See workflowConsentNotice for why this is not derived
			// from the script.
			desc := workflowPermissionDescription(params.Description)

			p, err := c.permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        c.cfg.WorkingDir(),
				ToolCallID:  call.ID,
				ToolName:    WorkflowToolName,
				Action:      "execute",
				Description: desc,
				Params:      params,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return tools.NewPermissionDeniedResponse(), nil
			}

			spawn := func(ctx context.Context, index int, label, prompt string, opts workflow.SpawnOpts) (string, error) {
				title := label
				if title == "" {
					title = fmt.Sprintf("Workflow Agent %d", index+1)
				}

				// Select the agent by requested profile AND model.
				agent, err := agents.get(workflowSpawnKey(opts))
				if err != nil {
					return "", err
				}

				subParams := subAgentParams{
					Agent:          agent,
					SessionID:      sessionID,
					AgentMessageID: agentMessageID,
					ToolCallID:     fmt.Sprintf("%s-a%d", call.ID, index),
					Prompt:         prompt,
					SessionTitle:   title,
				}

				// Coder-profile subagents auto-approve their
				// session so that writes don't prompt individually
				// (the single workflow permission covers them).
				// Task-profile subagents are read-only and don't
				// need auto-approval.
				if opts.Agent == workflowProfileCoder {
					subParams.SessionSetup = func(id string) {
						c.permissions.AutoApproveSession(id)
					}
				}

				resp, err := c.runSubAgent(ctx, subParams)
				if err != nil {
					return "", err
				}
				if resp.IsError {
					return "", errors.New(resp.Content)
				}
				return resp.Content, nil
			}

			progressFn := func(p workflow.Progress) {
				c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
					SessionID:  sessionID,
					Type:       notify.TypeWorkflowProgress,
					ProviderID: "",
					WorkflowProgress: &notify.WorkflowProgress{
						ToolCallID: call.ID,
						Kind:       p.Kind,
						Index:      p.Index,
						Label:      p.Label,
						Message:    p.Message,
						Running:    p.Running,
						Completed:  p.Completed,
						Total:      p.Total,
					},
				})
			}

			result, err := workflow.Run(ctx, params.Script, spawn, workflow.Options{
				Progress: progressFn,
			})
			if err != nil {
				if ctx.Err() != nil {
					return fantasy.ToolResponse{}, err
				}

				logsStr := "(none)"
				if len(result.Logs) > 0 {
					var sb strings.Builder
					for _, l := range result.Logs {
						sb.WriteString("- ")
						sb.WriteString(l)
						sb.WriteString("\n")
					}
					logsStr = sb.String()
				}
				return fantasy.WithResponseMetadata(
					fantasy.NewTextErrorResponse(fmt.Sprintf("workflow failed: %v\n\nLogs:\n%s", err, logsStr)),
					map[string]any{"agents": result.AgentCount, "logs": result.Logs},
				), nil
			}

			var valStr string
			if result.Value == "" {
				valStr = "null"
			} else {
				valStr = result.Value
			}

			logsStr := "(none)"
			if len(result.Logs) > 0 {
				var sb strings.Builder
				for _, l := range result.Logs {
					sb.WriteString("- ")
					sb.WriteString(l)
					sb.WriteString("\n")
				}
				logsStr = sb.String()
			}

			out := fmt.Sprintf("Workflow finished: %d agent(s) run.\n\nReturn value:\n```json\n%s\n```\n\nLogs:\n%s", result.AgentCount, valStr, logsStr)

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(out),
				map[string]any{"agents": result.AgentCount, "logs": result.Logs},
			), nil
		},
	), nil
}

// workflowConsentNotice is appended to every workflow permission
// prompt. It states the capability the approval grants rather than
// anything about the script, because the script is a Turing-complete
// Lua program and any claim derived from reading it can be made
// false.
//
// This replaced a substring scan (countCoderAgents) that counted
// `agent = "coder"` occurrences and reported the number to the user.
// That count was wrong for ordinary Lua, not just for deliberate
// evasion: `{agent  =  "coder"}` (two spaces), `{["agent"]="coder"}`,
// and a newline between the key and the value all spawn a
// write-capable sub-agent while the scan reports zero, so the user
// could approve materially more than the prompt showed. The sentence
// below is true for every script, including those five, so no script
// can falsify it.
//
// The corresponding grant is the AutoApproveSession call in the spawn
// closure above: a coder-profile sub-agent's session is auto-approved
// for its lifetime, so its edit/write/bash calls never prompt again.
//
// The notice deliberately claims NO directory scope. permissionService.Request
// consults autoApproveSessions[opts.SessionID] and returns true before it ever
// reads opts.Path (internal/permission/permission.go:245-256), and neither the
// write tool nor the bash tool constrains a path to the working directory --
// bash even takes an arbitrary working_dir. So an auto-approved sub-agent may
// write anywhere the process can. Saying "in this directory" would have been a
// bound the code does not enforce, which is the same class of understatement as
// the count this replaced. TestWorkflowConsentNoticeMatchesRealGrantScope pins
// the sentence to that behaviour.
const workflowConsentNotice = "Approving this workflow grants every sub-agent it starts file-write and shell access, on any path this process can reach, for the duration of the workflow, without prompting again for each action."

// workflowPermissionDescription builds the text shown in the workflow
// permission dialog: the model's own description of the workflow,
// followed by the capability being granted.
func workflowPermissionDescription(description string) string {
	if description == "" {
		return workflowConsentNotice
	}
	return description + "\n\n" + workflowConsentNotice
}
