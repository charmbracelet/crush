package agent

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode"

	"charm.land/fantasy"
)

// toolErrorBoundary wraps a fantasy.AgentTool so a Go error returned from
// Run reaches the model as an error tool result instead of aborting the
// turn.
//
// fantasy treats any error returned from AgentTool.Run as critical: the
// whole step is discarded, the agent loop returns the error, and the model
// never sees what went wrong. (Before fantasy v0.45.1 it was worse: any
// error matching net.Error, which includes every wrapped syscall.Errno,
// also tripped the retry loop, so a view of a path under a regular file
// cost 3 retries and 4 model requests before "Provider Error".) Most tool
// errors in crush are ordinary failures the model can recover from (an
// invalid enum value, an unreachable URL, a path it cannot write), so they
// are converted into error responses here. The error text is unchanged;
// any output the tool produced before failing is kept ahead of it.
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
	if ctxErr := ctx.Err(); ctxErr != nil {
		// The run was canceled or timed out. Let fantasy abort the turn so
		// the cancellation is recorded as such.
		if errors.Is(err, ctxErr) {
			return resp, err
		}
		// The tool reported something else, but the user has already
		// canceled. Join the cancellation in so crush and fantasy classify
		// the abort as a cancel while the tool's own text is kept.
		return resp, errors.Join(ctxErr, err)
	}
	// Debug, not Warn: a tool error is now an ordinary result the model
	// handles, and its text is already in the session.
	slog.Debug("Tool returned an error; reporting it to the model",
		"tool", call.Name,
		"tool_call_id", call.ID,
		"error", err,
	)
	// Keep any output the tool produced before it failed, ahead of the
	// error, so the model sees both.
	content := err.Error()
	if partial := strings.TrimRightFunc(resp.Content, unicode.IsSpace); partial != "" {
		content = partial + "\n\n" + content
	}
	out := fantasy.NewTextErrorResponse(content)
	// Keep whatever the inner tool decided about the turn and its metadata
	// (a hook halt, for example) so the error result behaves like any other
	// error response from that tool.
	out.StopTurn = resp.StopTurn
	out.Metadata = resp.Metadata
	return out, nil
}
