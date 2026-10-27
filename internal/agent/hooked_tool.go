package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/tidwall/sjson"
)

// hookedTool wraps a fantasy.AgentTool to run PreToolUse hooks before the
// inner tool and PostToolUse or PostToolUseFailure hooks after it.
type hookedTool struct {
	inner       fantasy.AgentTool
	runner      *hooks.Runner
	isSubAgent  bool
}

func newHookedTool(inner fantasy.AgentTool, runner *hooks.Runner, isSubAgent bool) *hookedTool {
	return &hookedTool{inner: inner, runner: runner, isSubAgent: isSubAgent}
}

// wrapToolsWithHooks returns a tool slice with each entry wrapped in a
// hookedTool. Returns the original slice unchanged when runner is nil.
//
// For top-level agents (isSubAgent=false), all matching hooks run.
// For sub-agents (isSubAgent=true), only hooks that have
// IncludeSubAgents=true run. This lets users opt into sub-agent
// interception for audit logging, secret redaction, or policy enforcement.
func wrapToolsWithHooks(tools []fantasy.AgentTool, runner *hooks.Runner, isSubAgent bool, hookCfgs map[string][]hooks.HookConfig) []fantasy.AgentTool {
	if runner == nil {
		return tools
	}
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = newHookedTool(tool, runner, isSubAgent)
	}
	return out
}

// anyHookIncludesSubAgents reports whether any configured hook across all
// events has IncludeSubAgents set. This is a fast pre-check before deciding
// whether to wrap sub-agent tools.
func anyHookIncludesSubAgents(hookCfgs map[string][]hooks.HookConfig) bool {
	for _, hooks := range hookCfgs {
		for _, h := range hooks {
			if h.IncludeSubAgents {
				return true
			}
		}
	}
	return false
}

func (h *hookedTool) Info() fantasy.ToolInfo {
	return h.inner.Info()
}

func (h *hookedTool) ProviderOptions() fantasy.ProviderOptions {
	return h.inner.ProviderOptions()
}

func (h *hookedTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	h.inner.SetProviderOptions(opts)
}

func (h *hookedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	ev := hooks.Event{
		Name:       hooks.EventPreToolUse,
		SessionID:  tools.GetSessionFromContext(ctx),
		IsSubAgent: h.isSubAgent,
		Tool:       &hooks.ToolCall{ID: call.ID, Name: call.Name, Input: call.Input},
	}

	pre := h.runner.Run(ctx, ev)
	if pre.Blocked() {
		reason := fmt.Sprintf("Tool call blocked by hook. Reason: %s", pre.Reason)
		if pre.Halt {
			reason = fmt.Sprintf("Turn halted by hook. Reason: %s", pre.Reason)
		}
		resp := fantasy.NewTextErrorResponse(reason)
		// Halt ends the whole turn; a plain deny only blocks this tool
		// call so the model can see the error and try something else.
		resp.StopTurn = pre.Halt
		resp.Metadata = mergeHookMetadata("", pre)
		return resp, nil
	}
	if pre.UpdatedInput != "" {
		call.Input = pre.UpdatedInput
		ev.Tool.Input = pre.UpdatedInput
	}
	// An explicit allow from a hook pre-approves the permission prompt for
	// this tool call. Silence falls through to the normal permission flow.
	if pre.Decision == hooks.DecisionAllow {
		ctx = permission.WithHookApproval(ctx, call.ID)
	}

	resp, err := h.inner.Run(ctx, call)

	ev = postToolEvent(ev, resp, err)
	post := h.runner.Run(ctx, ev)
	if post.UpdatedOutput != "" && ev.Name == hooks.EventPostToolUse {
		resp.Content = post.UpdatedOutput
	}
	resp.StopTurn = resp.StopTurn || post.Halt
	resp.Metadata = mergeHookMetadata(resp.Metadata, pre, post)
	if err != nil {
		// A Go error aborts the run, so there is no result to carry
		// feedback to the model.
		return resp, err
	}

	// A post hook that blocks cannot undo the call, so its reason is
	// handed to the model as feedback instead, as in Claude Code.
	feedback := []string{pre.Context, post.Context}
	if post.Blocked() && post.Reason != "" {
		feedback = append(feedback, "Hook feedback: "+post.Reason)
	}
	resp.Content = joinNonEmpty(resp.Content, feedback...)
	return resp, nil
}

// postToolEvent turns the PreToolUse event into the one describing how
// the call ended. A Go error and an error response both count as a
// failure, as in Claude Code, so PostToolUse only ever sees output the
// model will treat as a success.
func postToolEvent(ev hooks.Event, resp fantasy.ToolResponse, err error) hooks.Event {
	switch {
	case err != nil:
		ev.Name, ev.Error = hooks.EventPostToolUseFailure, err.Error()
	case resp.IsError:
		ev.Name, ev.Error = hooks.EventPostToolUseFailure, resp.Content
	default:
		ev.Name, ev.ToolResponse = hooks.EventPostToolUse, new(resp.Content)
	}
	return ev
}

// joinNonEmpty appends the non-empty parts to content, one per line.
func joinNonEmpty(content string, parts ...string) string {
	for _, p := range parts {
		if p == "" {
			continue
		}
		if content != "" {
			content += "\n"
		}
		content += p
	}
	return content
}

// mergeHookMetadata injects the combined hook metadata for the given
// results into existing tool metadata under the "hook" key. Results
// with no hooks are skipped; when nothing ran, existing is returned
// unchanged.
func mergeHookMetadata(existing string, results ...hooks.AggregateResult) string {
	var meta hooks.HookMetadata
	var decision hooks.Decision
	for _, r := range results {
		if r.HookCount == 0 {
			continue
		}
		meta.HookCount += r.HookCount
		meta.Hooks = append(meta.Hooks, r.Hooks...)
		meta.Halt = meta.Halt || r.Halt
		meta.InputRewrite = meta.InputRewrite || r.UpdatedInput != ""
		meta.OutputRewrite = meta.OutputRewrite || r.UpdatedOutput != ""
		// Decisions are ordered none < allow < deny, so the strongest wins.
		decision = max(decision, r.Decision)
		meta.Reason = strings.TrimSpace(meta.Reason + "\n" + r.Reason)
	}
	if meta.HookCount == 0 {
		return existing
	}
	meta.Decision = decision.String()
	data, err := json.Marshal(meta)
	if err != nil {
		return existing
	}
	if existing == "" {
		existing = "{}"
	}
	merged, err := sjson.SetRaw(existing, "hook", string(data))
	if err != nil {
		return existing
	}
	return merged
}
