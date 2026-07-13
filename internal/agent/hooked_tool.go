package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/tidwall/sjson"
)

// hookedTool wraps a fantasy.AgentTool to run hook policy before and after
// delegating to the inner tool.
type hookedTool struct {
	inner             fantasy.AgentTool
	preRunner         *hooks.Runner
	postRunner        *hooks.Runner
	postFailureRunner *hooks.Runner
}

func newHookedTool(inner fantasy.AgentTool, preRunner, postRunner, postFailureRunner *hooks.Runner) *hookedTool {
	return &hookedTool{inner: inner, preRunner: preRunner, postRunner: postRunner, postFailureRunner: postFailureRunner}
}

// wrapToolsWithHooks returns a tool slice with each entry wrapped in a
// hookedTool. Returns the original slice unchanged when runner is nil or
// when isSubAgent is true — sub-agents never fire hooks, the top-level
// invocation of the sub-agent tool itself is wrapped on the caller's side.
func wrapToolsWithHooks(tools []fantasy.AgentTool, preRunner, postRunner, postFailureRunner *hooks.Runner, isSubAgent bool) []fantasy.AgentTool {
	if isSubAgent || preRunner == nil && postRunner == nil && postFailureRunner == nil {
		return tools
	}
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = newHookedTool(tool, preRunner, postRunner, postFailureRunner)
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

func (h *hookedTool) ResolveDeferredTools(names []string) []fantasy.AgentTool {
	provider, ok := h.inner.(tools.DeferredToolProvider)
	if !ok {
		return nil
	}
	return wrapToolsWithHooks(
		provider.ResolveDeferredTools(names),
		h.preRunner,
		h.postRunner,
		h.postFailureRunner,
		false,
	)
}

func (h *hookedTool) PollutesMemory() bool {
	pollutingTool, ok := h.inner.(interface{ PollutesMemory() bool })
	return ok && pollutingTool.PollutesMemory()
}

func (h *hookedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := tools.GetSessionFromContext(ctx)
	var result hooks.AggregateResult
	if h.preRunner != nil {
		var err error
		result, err = h.preRunner.Run(ctx, hooks.EventPreToolUse, sessionID, call.Name, call.Input)
		if err != nil {
			slog.Warn("Hook execution error, proceeding with tool call",
				"tool", call.Name, "error", err)
		}

		if result.Decision == hooks.DecisionDeny || result.Decision == hooks.DecisionBlock || result.Halt {
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

	failed := resp.IsError
	runner := h.postRunner
	eventName := hooks.EventPostToolUse
	if failed && h.postFailureRunner != nil {
		runner = h.postFailureRunner
		eventName = hooks.EventPostToolUseFailure
	}
	if runner != nil {
		postPayload, _ := json.Marshal(map[string]any{
			"content":   resp.Content,
			"is_error":  resp.IsError,
			"metadata":  resp.Metadata,
			"stop_turn": resp.StopTurn,
		})
		postResult, postErr := runner.RunPayload(ctx, eventName, sessionID, call.Name, call.Input, hooks.Payload{
			ToolName:   call.Name,
			ToolInput:  json.RawMessage(call.Input),
			ToolResult: json.RawMessage(postPayload),
		})
		if postErr != nil {
			slog.Warn(eventName+" hook execution error, proceeding with tool result",
				"tool", call.Name, "error", postErr)
		}
		blockedByPostHook := postResult.Decision == hooks.DecisionDeny || postResult.Decision == hooks.DecisionBlock
		if blockedByPostHook {
			resp = fantasy.NewTextErrorResponse(postHookFeedback(postResult, eventName))
		}
		if postResult.Context != "" && !blockedByPostHook {
			if resp.Content != "" {
				resp.Content += "\n"
			}
			resp.Content += postResult.Context
		}
		if postResult.Halt {
			if postResult.Reason != "" {
				if resp.Content != "" {
					resp.Content += "\n"
				}
				resp.Content += "Turn halted by post-tool hook. Reason: " + postResult.Reason
			}
			resp.StopTurn = true
		}
		resp.Metadata = mergeHookMetadata(resp.Metadata, postResult)
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

func postHookFeedback(result hooks.AggregateResult, eventName string) string {
	var parts []string
	if result.Reason != "" {
		parts = append(parts, result.Reason)
	}
	if result.Context != "" && result.Context != result.Reason {
		parts = append(parts, result.Context)
	}
	if len(parts) == 0 {
		parts = append(parts, eventName+" hook blocked normal tool result processing.")
	}
	return strings.Join(parts, "\n")
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
