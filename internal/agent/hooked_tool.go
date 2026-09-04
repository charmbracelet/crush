package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/tidwall/sjson"
)

// hookedTool wraps a fantasy.AgentTool to run PreToolUse hooks before
// delegating to the inner tool.
type hookedTool struct {
	inner  fantasy.AgentTool
	runner *hooks.Runner
	// blockedInPlan returns true if the wrapped tool should be rejected
	// when the agent is running in plan mode. nil means "always allow".
	blockedInPlan func() bool
}

func newHookedTool(inner fantasy.AgentTool, runner *hooks.Runner) *hookedTool {
	return &hookedTool{inner: inner, runner: runner}
}

// newPlanBlockedHookedTool marks a tool as one that should be rejected in
// plan mode. The plan mode prompt is a read-only prompt; tools that mutate
// state (write, edit, bash, etc.) must be blocked so the agent cannot
// accidentally execute them.
func newPlanBlockedHookedTool(inner fantasy.AgentTool, runner *hooks.Runner) *hookedTool {
	t := newHookedTool(inner, runner)
	t.blockedInPlan = func() bool { return true }
	return t
}

// isToolBlockedInPlan reports whether a tool name is in the write set
// blocked by plan mode. It is the single source of truth for the plan
// guard, used by the hookedTool wrapper and tests.
func isToolBlockedInPlan(toolName string) bool {
	switch toolName {
	case "bash", "edit", "write", "multiedit",
		"download",
		"lsp_rename", "lsp_replace_symbol":
		return true
	}
	return false
}

// wrapToolsWithHooks returns a tool slice with each entry wrapped in a
// hookedTool. Returns the original slice unchanged when isSubAgent is
// true — sub-agents never fire hooks, the top-level invocation of the
// sub-agent tool itself is wrapped on the caller's side.
//
// The plan-mode guard is applied even when runner is nil. The plan
// prompt forbids state-mutating tools; that policy is a property of
// plan mode itself, not of the hook system, so it must be enforced
// regardless of whether the user has configured any PreToolUse hooks.
func wrapToolsWithHooks(tools []fantasy.AgentTool, runner *hooks.Runner, isSubAgent bool) []fantasy.AgentTool {
	if isSubAgent {
		return tools
	}
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		if isToolBlockedInPlan(tool.Info().Name) {
			out[i] = newPlanBlockedHookedTool(tool, runner)
		} else if runner != nil {
			out[i] = newHookedTool(tool, runner)
		} else {
			out[i] = tool
		}
	}
	return out
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
	// Plan mode guard: tools flagged as state-mutating are rejected with
	// a synthetic response so the model can read it and continue
	// planning instead of receiving an opaque error.
	if h.blockedInPlan != nil && h.blockedInPlan() && ModeFromContext(ctx) == AgentModePlan {
		resp := fantasy.NewTextErrorResponse(fmt.Sprintf(
			"[plan mode] %s skipped - no side effects in plan mode. Switch to Build (Shift+Tab) to execute.",
			call.Name,
		))
		return resp, nil
	}

	// When no hook runner is configured, the wrap is only here to enforce
	// the plan guard above. Skip the hook step and delegate directly.
	if h.runner == nil {
		return h.inner.Run(ctx, call)
	}

	sessionID := tools.GetSessionFromContext(ctx)
	result, err := h.runner.Run(ctx, hooks.EventPreToolUse, sessionID, call.Name, call.Input)
	if err != nil {
		slog.Warn("Hook execution error, proceeding with tool call",
			"tool", call.Name, "error", err)
	}

	if result.Decision == hooks.DecisionDeny || result.Halt {
		reason := fmt.Sprintf("Tool call blocked by hook. Reason: %s", result.Reason)
		if result.Halt {
			reason = fmt.Sprintf("Turn halted by hook. Reason: %s", result.Reason)
		}
		resp := fantasy.NewTextErrorResponse(reason)
		// Halt ends the whole turn; a plain deny only blocks this tool
		// call so the model can see the error and try something else.
		resp.StopTurn = result.Halt
		resp.Metadata = hookMetadataJSON(result)
		return resp, nil
	}

	if result.UpdatedInput != "" {
		call.Input = result.UpdatedInput
	}

	// An explicit allow from a hook pre-approves the permission prompt for
	// this tool call. Deny is already handled above; silence falls through
	// to the normal permission flow.
	if result.Decision == hooks.DecisionAllow {
		ctx = permission.WithHookApproval(ctx, call.ID)
	}

	resp, err := h.inner.Run(ctx, call)
	if err != nil {
		return resp, err
	}

	if result.Context != "" {
		if resp.Content != "" {
			resp.Content += "\n"
		}
		resp.Content += result.Context
	}

	resp.Metadata = mergeHookMetadata(resp.Metadata, result)
	return resp, nil
}

// buildHookMetadata creates a HookMetadata from an AggregateResult.
func buildHookMetadata(result hooks.AggregateResult) hooks.HookMetadata {
	return hooks.HookMetadata{
		HookCount:    result.HookCount,
		Decision:     result.Decision.String(),
		Halt:         result.Halt,
		Reason:       result.Reason,
		InputRewrite: result.UpdatedInput != "",
		Hooks:        result.Hooks,
	}
}

// hookMetadataJSON builds a JSON string containing only the hook metadata.
func hookMetadataJSON(result hooks.AggregateResult) string {
	meta := buildHookMetadata(result)
	data, err := json.Marshal(meta)
	if err != nil {
		return ""
	}
	return `{"hook":` + string(data) + `}`
}

// mergeHookMetadata injects hook metadata into existing tool metadata.
func mergeHookMetadata(existing string, result hooks.AggregateResult) string {
	if result.HookCount == 0 {
		return existing
	}
	meta := buildHookMetadata(result)
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
