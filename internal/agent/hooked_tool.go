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
// and PostToolUse hooks after delegating to the inner tool.
type hookedTool struct {
	inner fantasy.AgentTool
	pre   *hooks.Runner // nil = no PreToolUse hooks
	post  *hooks.Runner // nil = no PostToolUse hooks
}

func newHookedTool(inner fantasy.AgentTool, pre, post *hooks.Runner) *hookedTool {
	return &hookedTool{inner: inner, pre: pre, post: post}
}

// wrapToolsWithHooks returns a tool slice with each entry wrapped in a
// hookedTool. Returns the original slice unchanged when both runners are
// nil or when isSubAgent is true — sub-agents never fire hooks; the
// top-level invocation of the sub-agent tool itself is wrapped on the
// caller's side.
func wrapToolsWithHooks(tools []fantasy.AgentTool, pre, post *hooks.Runner, isSubAgent bool) []fantasy.AgentTool {
	if (pre == nil && post == nil) || isSubAgent {
		return tools
	}
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = newHookedTool(tool, pre, post)
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
	sessionID := tools.GetSessionFromContext(ctx)
	var result hooks.AggregateResult

	if h.pre != nil {
		var err error
		result, err = h.pre.Run(ctx, hooks.EventPreToolUse, sessionID, call.Name, call.Input)
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

	if h.pre != nil {
		resp.Metadata = mergeHookMetadata(resp.Metadata, result)
	}

	if h.post != nil {
		postAgg, perr := h.post.RunEvent(ctx, hooks.EventInput{
			Event:     hooks.EventPostToolUse,
			SessionID: sessionID,
			ToolName:  call.Name,
			ToolInput: call.Input,
			ToolResponse: &hooks.ToolResponsePayload{
				Content: resp.Content,
				IsError: resp.IsError,
			},
		})
		if perr != nil {
			slog.Warn("PostToolUse hook error, ignoring", "tool", call.Name, "error", perr)
		}
		if postAgg.Context != "" {
			if resp.Content != "" {
				resp.Content += "\n"
			}
			resp.Content += postAgg.Context
		}
		if postAgg.Decision == hooks.DecisionDeny && postAgg.Reason != "" {
			resp.Content += "\n[post-hook] " + postAgg.Reason
		}
		if postAgg.Halt {
			resp.StopTurn = true
		}
		if postAgg.UpdatedInput != "" || postAgg.UpdatedPrompt != "" {
			slog.Warn("PostToolUse hook rewrite fields ignored", "tool", call.Name)
		}
		resp.Metadata = mergeHookMetadataKey(resp.Metadata, postAgg, "hook_post")
	}

	return resp, nil
}

func hookMetadataJSON(result hooks.AggregateResult) string {
	meta := hooks.HookMetadata{
		HookCount:    result.HookCount,
		Decision:     result.Decision.String(),
		Halt:         result.Halt,
		Reason:       result.Reason,
		InputRewrite: result.UpdatedInput != "",
		Hooks:        result.Hooks,
	}
	b, err := json.Marshal(map[string]any{"hook": meta})
	if err != nil {
		return ""
	}
	return string(b)
}

func mergeHookMetadata(existing string, result hooks.AggregateResult) string {
	return mergeHookMetadataKey(existing, result, "hook")
}

func mergeHookMetadataKey(existing string, result hooks.AggregateResult, key string) string {
	// No hooks matched this tool: leave the metadata untouched so the UI
	// hook indicator only appears for calls a hook actually processed.
	if result.HookCount == 0 {
		return existing
	}
	meta := hooks.HookMetadata{
		HookCount:    result.HookCount,
		Decision:     result.Decision.String(),
		Halt:         result.Halt,
		Reason:       result.Reason,
		InputRewrite: result.UpdatedInput != "",
		Hooks:        result.Hooks,
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return existing
	}
	if existing == "" {
		existing = "{}"
	}
	out, err := sjson.SetRaw(existing, key, string(b))
	if err != nil {
		return existing
	}
	return out
}
