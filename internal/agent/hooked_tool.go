package agent

import (
	"context"
	"log/slog"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/hooks"
)

// hookedTool wraps a fantasy.AgentTool to run PreToolUse hooks before
// delegating to the inner tool.
type hookedTool struct {
	inner  fantasy.AgentTool
	runner *hooks.Runner
}

func newHookedTool(inner fantasy.AgentTool, runner *hooks.Runner) *hookedTool {
	return &hookedTool{inner: inner, runner: runner}
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
	result, err := h.runner.Run(ctx, hooks.EventPreToolUse, sessionID, call.Name, call.Input)
	if err != nil {
		slog.Warn("Hook execution error, proceeding with tool call",
			"tool", call.Name, "error", err)
	}

	if result.Decision == hooks.DecisionDeny {
		return fantasy.NewTextErrorResponse(result.Reason), nil
	}

	if result.UpdatedInput != "" {
		call.Input = result.UpdatedInput
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

	return resp, nil
}
