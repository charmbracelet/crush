package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type boundaryParams struct {
	X int `json:"x"`
}

// boundaryTool builds a tool whose Run delegates to fn.
func boundaryTool(name string, fn func(ctx context.Context) (fantasy.ToolResponse, error)) fantasy.AgentTool {
	return fantasy.NewAgentTool(name, "test tool", func(ctx context.Context, _ boundaryParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fn(ctx)
	})
}

// --- Unit tests on the wrapper ---

func TestToolErrorBoundary_ConvertsGoErrorIntoErrorResponse(t *testing.T) {
	t.Parallel()

	inner := boundaryTool("boom", func(context.Context) (fantasy.ToolResponse, error) {
		return fantasy.ToolResponse{}, errors.New(`invalid status "done" for todo "x"`)
	})
	wrapped := newToolErrorBoundary(inner)

	resp, err := wrapped.Run(t.Context(), fantasy.ToolCall{ID: "tc1", Name: "boom", Input: `{"x":1}`})
	require.NoError(t, err, "a tool failure must not surface as a Go error")
	require.True(t, resp.IsError)
	require.Equal(t, `invalid status "done" for todo "x"`, resp.Content, "the error text must reach the model unchanged")
}

func TestToolErrorBoundary_PropagatesErrorWhenRunIsCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	inner := boundaryTool("boom", func(ctx context.Context) (fantasy.ToolResponse, error) {
		return fantasy.ToolResponse{}, ctx.Err()
	})
	wrapped := newToolErrorBoundary(inner)

	_, err := wrapped.Run(ctx, fantasy.ToolCall{ID: "tc1", Name: "boom", Input: `{"x":1}`})
	require.ErrorIs(t, err, context.Canceled, "cancellation must still abort the turn")
}

func TestToolErrorBoundary_PassesSuccessThrough(t *testing.T) {
	t.Parallel()

	inner := boundaryTool("ok", func(context.Context) (fantasy.ToolResponse, error) {
		return fantasy.WithResponseMetadata(fantasy.NewTextResponse("done"), map[string]string{"k": "v"}), nil
	})
	wrapped := newToolErrorBoundary(inner)

	resp, err := wrapped.Run(t.Context(), fantasy.ToolCall{ID: "tc1", Name: "ok", Input: `{"x":1}`})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Equal(t, "done", resp.Content)
	require.JSONEq(t, `{"k":"v"}`, resp.Metadata)
}

func TestToolErrorBoundary_KeepsStopTurnAndMetadataOnError(t *testing.T) {
	t.Parallel()

	inner := boundaryTool("boom", func(context.Context) (fantasy.ToolResponse, error) {
		return fantasy.ToolResponse{StopTurn: true, Metadata: `{"hook":{"halt":true}}`}, errors.New("halted")
	})
	wrapped := newToolErrorBoundary(inner)

	resp, err := wrapped.Run(t.Context(), fantasy.ToolCall{ID: "tc1", Name: "boom", Input: `{"x":1}`})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.True(t, resp.StopTurn, "the inner tool's decision to stop the turn must survive")
	require.JSONEq(t, `{"hook":{"halt":true}}`, resp.Metadata)
}

func TestToolErrorBoundary_DelegatesInfoAndProviderOptions(t *testing.T) {
	t.Parallel()

	inner := fantasy.NewParallelAgentTool("par", "parallel tool", func(context.Context, boundaryParams, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})
	wrapped := newToolErrorBoundary(inner)

	require.Equal(t, inner.Info(), wrapped.Info())
	require.True(t, wrapped.Info().Parallel, "the Parallel flag must survive wrapping")

	opts := fantasy.ProviderOptions{}
	wrapped.SetProviderOptions(opts)
	require.NotNil(t, inner.ProviderOptions())
	require.Equal(t, inner.ProviderOptions(), wrapped.ProviderOptions())
}

func TestWrapToolsWithErrorBoundary(t *testing.T) {
	t.Parallel()

	require.Nil(t, wrapToolsWithErrorBoundary(nil))
	require.Empty(t, wrapToolsWithErrorBoundary([]fantasy.AgentTool{}))

	a := boundaryTool("a", func(context.Context) (fantasy.ToolResponse, error) { return fantasy.NewTextResponse("a"), nil })
	b := boundaryTool("b", func(context.Context) (fantasy.ToolResponse, error) { return fantasy.NewTextResponse("b"), nil })
	wrapped := wrapToolsWithErrorBoundary([]fantasy.AgentTool{a, b})

	require.Len(t, wrapped, 2)
	for i, want := range []string{"a", "b"} {
		boundary, ok := wrapped[i].(*toolErrorBoundary)
		require.True(t, ok, "every tool must be wrapped")
		require.Equal(t, want, boundary.Info().Name)
	}
}

// --- Integration tests through sessionAgent.Run ---

// toolCallOnceModel streams a single call to toolName on its first request
// and a plain text completion on every request after that. A run that
// recovers from the tool result therefore ends with end_turn after two
// requests; a run that aborts ends after one.
type toolCallOnceModel struct {
	calls    atomic.Int32
	toolName string
	input    string
}

func (m *toolCallOnceModel) Provider() string { return "fake" }
func (m *toolCallOnceModel) Model() string    { return "fake-model" }

func (m *toolCallOnceModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "done"}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *toolCallOnceModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	n := m.calls.Add(1)
	return func(yield func(fantasy.StreamPart) bool) {
		if n == 1 {
			id := "tc1"
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: id, ToolCallName: m.toolName}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputDelta, ID: id, Delta: m.input}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: id}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: m.toolName, ToolCallInput: m.input}) {
				return
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "recovered"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *toolCallOnceModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *toolCallOnceModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

type boundaryRun struct {
	llmCalls    int32
	finish      *message.Finish
	toolResults []message.ToolResult
	runErr      error
	elapsed     time.Duration
}

// boundaryHarness runs one prompt through sessionAgent.Run with the given
// tool list and reports what was persisted. The tool list is passed as-is
// so tests can compare wrapped and unwrapped behavior. The environment,
// agent, and session ID are exposed through the setup callback for tools
// that need them (a real tool backed by the test database, or a test that
// cancels the run from inside the tool).
type boundaryHarness struct {
	env       fakeEnv
	agent     *sessionAgent
	sessionID string
}

func runBoundaryHarness(t *testing.T, toolName, input string, buildTool func(h *boundaryHarness) fantasy.AgentTool, wrap bool) boundaryRun {
	t.Helper()

	env := testEnv(t)
	h := &boundaryHarness{env: env}
	toolList := []fantasy.AgentTool{buildTool(h)}
	if wrap {
		toolList = wrapToolsWithErrorBoundary(toolList)
	}
	model := &toolCallOnceModel{toolName: toolName, input: input}
	// Title generation runs on the small model. It must be a separate
	// instance, or it would consume the tool-call turn of the large model.
	h.agent = testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system", toolList...).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "boundary")
	require.NoError(t, err)
	h.sessionID = sess.ID

	start := time.Now()
	_, runErr := h.agent.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "go"})
	out := boundaryRun{llmCalls: model.calls.Load(), runErr: runErr, elapsed: time.Since(start)}

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	for _, m := range msgs {
		switch m.Role {
		case message.Assistant:
			if f := m.FinishPart(); f != nil {
				out.finish = f
			}
		case message.Tool:
			out.toolResults = append(out.toolResults, m.ToolResults()...)
		}
	}
	require.NotNil(t, out.finish, "the assistant message must carry a finish part")
	return out
}

func TestToolErrorBoundary_ModelRecoversFromToolError(t *testing.T) {
	t.Parallel()

	res := runBoundaryHarness(t, "boom", `{"x":1}`, func(*boundaryHarness) fantasy.AgentTool {
		return boundaryTool("boom", func(context.Context) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, errors.New(`invalid status "done" for todo "x"`)
		})
	}, true)

	require.NoError(t, res.runErr)
	require.Equal(t, int32(2), res.llmCalls, "the model must get a second request that includes the error")
	require.Equal(t, message.FinishReasonEndTurn, res.finish.Reason)
	require.Len(t, res.toolResults, 1)
	require.True(t, res.toolResults[0].IsError)
	require.Equal(t, `invalid status "done" for todo "x"`, res.toolResults[0].Content)
}

// TestToolErrorBoundary_UnwrappedGoErrorAbortsTurn documents the behavior
// the boundary exists to fix: without it, fantasy treats the same error as
// critical, the model is never called again, and the turn ends with a
// generic error finish.
func TestToolErrorBoundary_UnwrappedGoErrorAbortsTurn(t *testing.T) {
	t.Parallel()

	res := runBoundaryHarness(t, "boom", `{"x":1}`, func(*boundaryHarness) fantasy.AgentTool {
		return boundaryTool("boom", func(context.Context) (fantasy.ToolResponse, error) {
			return fantasy.ToolResponse{}, errors.New(`invalid status "done" for todo "x"`)
		})
	}, false)

	require.Error(t, res.runErr)
	require.Equal(t, int32(1), res.llmCalls)
	require.Equal(t, message.FinishReasonError, res.finish.Reason)
	require.Equal(t, "Provider Error", res.finish.Message)
}

func TestToolErrorBoundary_NetworkErrorDoesNotTriggerModelRetries(t *testing.T) {
	t.Parallel()

	res := runBoundaryHarness(t, "boom", `{"x":1}`, func(*boundaryHarness) fantasy.AgentTool {
		return boundaryTool("boom", func(ctx context.Context) (fantasy.ToolResponse, error) {
			// Same shape as download.go: a *url.Error wrapping a net.Error,
			// which fantasy's retry loop would otherwise treat as retryable
			// and replay the whole step (model request included) for.
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1/", nil)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
			if resp != nil {
				resp.Body.Close()
			}
			return fantasy.ToolResponse{}, fmt.Errorf("failed to download from URL: %w", err)
		})
	}, true)

	require.NoError(t, res.runErr)
	require.Equal(t, int32(2), res.llmCalls, "no retry: one request to get the call, one to recover")
	require.Less(t, res.elapsed, 5*time.Second, "no exponential backoff must run for a tool failure")
	require.Equal(t, message.FinishReasonEndTurn, res.finish.Reason)
	require.Len(t, res.toolResults, 1)
	require.True(t, res.toolResults[0].IsError)
	require.Contains(t, res.toolResults[0].Content, "connection refused")
}

func TestToolErrorBoundary_UserCancelDuringToolStillCancelsTurn(t *testing.T) {
	t.Parallel()

	res := runBoundaryHarness(t, "boom", `{"x":1}`, func(h *boundaryHarness) fantasy.AgentTool {
		return boundaryTool("boom", func(ctx context.Context) (fantasy.ToolResponse, error) {
			// Simulate the user canceling while the tool is running, the
			// same path the TUI takes on Escape.
			h.agent.Cancel(h.sessionID)
			<-ctx.Done()
			return fantasy.ToolResponse{}, ctx.Err()
		})
	}, true)

	require.ErrorIs(t, res.runErr, context.Canceled)
	require.Equal(t, int32(1), res.llmCalls)
	require.Equal(t, message.FinishReasonCanceled, res.finish.Reason)
}

func TestToolErrorBoundary_RealTodosToolBadStatus(t *testing.T) {
	t.Parallel()

	input := `{"todos":[{"content":"write tests","status":"done","active_form":"Writing tests"}]}`
	res := runBoundaryHarness(t, tools.TodosToolName, input, func(h *boundaryHarness) fantasy.AgentTool {
		// The tool must share the harness database so the session exists.
		return tools.NewTodosTool(h.env.sessions)
	}, true)

	require.NoError(t, res.runErr)
	require.Equal(t, int32(2), res.llmCalls)
	require.Equal(t, message.FinishReasonEndTurn, res.finish.Reason)
	require.Len(t, res.toolResults, 1)
	require.True(t, res.toolResults[0].IsError)
	require.Contains(t, res.toolResults[0].Content, `invalid status "done"`)
}
