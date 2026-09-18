package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/tidwall/sjson"
)

// hookedTool wraps a fantasy.AgentTool to run PreToolUse hooks before
// the tool runs and PostToolUse hooks after it returns.
type hookedTool struct {
	inner      fantasy.AgentTool
	dispatcher *hooks.Dispatcher
}

func newHookedTool(inner fantasy.AgentTool, dispatcher *hooks.Dispatcher) *hookedTool {
	return &hookedTool{inner: inner, dispatcher: dispatcher}
}

// wrapToolsWithHooks returns the slice unchanged when there are no hooks or
// when isSubAgent is set: a sub-agent's own tool calls never fire hooks (only
// the top-level call that spawns it does).
func wrapToolsWithHooks(allTools []fantasy.AgentTool, dispatcher *hooks.Dispatcher, isSubAgent bool) []fantasy.AgentTool {
	if dispatcher == nil || isSubAgent {
		return allTools
	}
	out := make([]fantasy.AgentTool, len(allTools))
	for i, tool := range allTools {
		out[i] = newHookedTool(tool, dispatcher)
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
	pre := h.dispatcher.Run(ctx, hooks.EventPreToolUse, hooks.EventData{
		SessionID: sessionID,
		ToolName:  call.Name,
		ToolInput: call.Input,
	})

	ctx, call, blocked := gateCall(ctx, call, pre)
	if blocked != nil {
		return *blocked, nil
	}

	resp, err := h.inner.Run(ctx, call)
	post := h.reportPostToolUse(ctx, call, resp, err)

	resp.Content = appendHookContext(resp.Content, pre.Context, post.Context)
	resp.Metadata = mergeHookMetadata(resp.Metadata, pre)
	resp.Metadata = mergeHookMetadata(resp.Metadata, post)
	return resp, err
}

// gateCall applies a PreToolUse result before the call runs. A non-nil
// response means the call must not run.
func gateCall(
	ctx context.Context,
	call fantasy.ToolCall,
	pre hooks.AggregateResult,
) (context.Context, fantasy.ToolCall, *fantasy.ToolResponse) {
	if pre.Decision == hooks.DecisionDeny || pre.Halt {
		blocked := blockedResponse(pre)
		return ctx, call, &blocked
	}

	if pre.UpdatedInput != "" {
		call.Input = pre.UpdatedInput
	}

	// An allow pre-approves the permission prompt; silence falls through to the
	// normal flow.
	if pre.Decision == hooks.DecisionAllow {
		ctx = permission.WithHookApproval(ctx, call.ID)
	}
	return ctx, call, nil
}

// reportPostToolUse tells PostToolUse hooks about a finished call, failures
// included, so a consumer sees the tool settle either way.
func (h *hookedTool) reportPostToolUse(
	ctx context.Context,
	call fantasy.ToolCall,
	resp fantasy.ToolResponse,
	callErr error,
) hooks.AggregateResult {
	observed := resp.Content
	if callErr != nil {
		observed = callErr.Error()
	}
	return h.dispatcher.Run(ctx, hooks.EventPostToolUse, hooks.EventData{
		SessionID:    tools.GetSessionFromContext(ctx),
		ToolName:     call.Name,
		ToolInput:    call.Input,
		ToolResponse: observed,
	})
}

func appendHookContext(content string, contexts ...string) string {
	for _, context := range contexts {
		if context == "" {
			continue
		}
		if content != "" {
			content += "\n"
		}
		content += context
	}
	return content
}

// blockedResponse marks a halt as turn-ending so the caller stops instead of
// letting the model retry; a deny only fails this one call.
func blockedResponse(result hooks.AggregateResult) fantasy.ToolResponse {
	reason := fmt.Sprintf("Tool call blocked by hook. Reason: %s", result.Reason)
	if result.Halt {
		reason = fmt.Sprintf("Turn halted by hook. Reason: %s", result.Reason)
	}
	resp := fantasy.NewTextErrorResponse(reason)
	resp.StopTurn = result.Halt
	resp.Metadata = hookMetadataJSON(result)
	return resp
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
