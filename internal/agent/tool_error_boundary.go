package agent

import (
	"context"
	"log/slog"

	"charm.land/fantasy"
)

// toolErrorBoundary wraps a fantasy.AgentTool so a Go error returned from
// Run reaches the model as an error tool result instead of aborting the
// turn.
//
// fantasy treats any error returned from AgentTool.Run as critical: the
// whole step is discarded, the agent loop returns the error, and the model
// never sees what went wrong. Errors that satisfy net.Error additionally
// trip fantasy's retry loop, which re-requests the model and re-runs the
// tool before giving up. That covers far more than network failures: a
// syscall.Errno implements both Timeout and Temporary, so any wrapped
// filesystem error (ENOENT, ENOTDIR, EACCES from os.Stat, os.ReadFile or
// os.WriteFile) matches net.Error through errors.As as well. Measured
// against fantasy v0.43.0 with a real model, a view of a path under a
// regular file cost 3 retries with 5s, 10s and 20s backoff and 4 model
// requests before the turn ended with "Provider Error". Most tool errors
// in crush are ordinary failures the model can recover from (an invalid
// enum value, an unreachable URL, a path it cannot write), so they are
// converted into error responses here. The text is unchanged; only the
// channel differs. Because the error never reaches fantasy, the retry
// loop never sees it either.
//
// Cancellation is the exception. When the run context is already done the
// error passes through untouched, so fantasy aborts the turn and the
// session records a canceled finish rather than a tool error.
type toolErrorBoundary struct {
	inner fantasy.AgentTool
}

func newToolErrorBoundary(inner fantasy.AgentTool) *toolErrorBoundary {
	return &toolErrorBoundary{inner: inner}
}

// wrapToolsWithErrorBoundary returns a copy of tools with every entry
// wrapped in a toolErrorBoundary. Apply it last so the boundary sits
// outside every other wrapper, hooks included. A nil or empty slice is
// returned unchanged.
func wrapToolsWithErrorBoundary(tools []fantasy.AgentTool) []fantasy.AgentTool {
	if len(tools) == 0 {
		return tools
	}
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = newToolErrorBoundary(tool)
	}
	return out
}

func (b *toolErrorBoundary) Info() fantasy.ToolInfo {
	return b.inner.Info()
}

func (b *toolErrorBoundary) ProviderOptions() fantasy.ProviderOptions {
	return b.inner.ProviderOptions()
}

func (b *toolErrorBoundary) SetProviderOptions(opts fantasy.ProviderOptions) {
	b.inner.SetProviderOptions(opts)
}

func (b *toolErrorBoundary) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	resp, err := b.inner.Run(ctx, call)
	if err == nil {
		return resp, nil
	}
	if ctx.Err() != nil {
		// The run was canceled or timed out. Let fantasy abort the turn so
		// the cancellation is recorded as such.
		return resp, err
	}
	slog.Warn("Tool returned an error; reporting it to the model",
		"tool", call.Name,
		"tool_call_id", call.ID,
		"error", err,
	)
	out := fantasy.NewTextErrorResponse(err.Error())
	// Keep whatever the inner tool decided about the turn and its metadata
	// (a hook halt, for example) so the error result behaves like any other
	// error response from that tool.
	out.StopTurn = resp.StopTurn
	out.Metadata = resp.Metadata
	return out, nil
}
