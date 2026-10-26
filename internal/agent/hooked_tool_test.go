package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// fakeTool records the context it was invoked with so tests can assert on
// values stamped onto it by the hookedTool decorator.
type fakeTool struct {
	name   string
	called bool
	gotCtx context.Context
	resp   fantasy.ToolResponse
	runErr error
}

func (f *fakeTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: f.name}
}

func (f *fakeTool) Run(ctx context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
	f.called = true
	f.gotCtx = ctx
	return f.resp, f.runErr
}

func (f *fakeTool) ProviderOptions() fantasy.ProviderOptions     { return nil }
func (f *fakeTool) SetProviderOptions(_ fantasy.ProviderOptions) {}

// newRunner builds a hooks.Runner from a single HookConfig, running the
// config-loader path that compiles the matcher regex.
func newRunner(t *testing.T, cmd string) *hooks.Runner {
	t.Helper()
	return newEventRunner(t, hooks.EventPreToolUse, cmd)
}

// newEventRunner builds a hooks.Runner for a single event's hooks.
func newEventRunner(t *testing.T, event, cmd string) *hooks.Runner {
	t.Helper()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			event: {{Command: cmd}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())
	return hooks.NewRunner(cfg.Hooks[event], t.TempDir(), t.TempDir())
}

func TestHookedTool_AllowStampsHookApproval(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	runner := newRunner(t, `echo '{"decision":"allow"}'`)
	tool := newHookedTool(inner, runner, nil)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-1", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called, "inner tool should have run")

	// The inner tool's permission service can now treat call-1 as pre-approved.
	svc := permission.NewPermissionService(t.TempDir(), false, nil)
	granted, err := svc.Request(inner.gotCtx, permission.CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "call-1",
		ToolName:   "view",
		Action:     "read",
		Path:       t.TempDir(),
	})
	require.NoError(t, err)
	require.True(t, granted, "hook allow should bypass the permission prompt")
}

func TestHookedTool_SilentDoesNotStampApproval(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	runner := newRunner(t, `exit 0`) // no stdout, no decision
	tool := newHookedTool(inner, runner, nil)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-2", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called)

	// With no hook opinion, a fresh permission request has nothing stamped
	// and must fall through to the normal flow. We verify by checking that
	// the context does not look pre-approved for this call ID: sending a
	// request that no subscriber resolves will block until cancelled.
	svc := permission.NewPermissionService(t.TempDir(), false, nil)
	ctx, cancel := context.WithCancel(inner.gotCtx)
	cancel()
	granted, err := svc.Request(ctx, permission.CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "call-2",
		ToolName:   "view",
		Action:     "read",
		Path:       t.TempDir(),
	})
	require.Error(t, err, "no approval stamped => request should reach the prompt path")
	require.False(t, granted)
}

func TestHookedTool_DenySkipsInnerTool(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash"}
	runner := newRunner(t, `echo "blocked" >&2; exit 2`)
	tool := newHookedTool(inner, runner, nil)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-3", Name: "bash"})
	require.NoError(t, err)
	require.False(t, inner.called, "denied call must not reach the inner tool")
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "blocked")
}

func TestWrapToolsWithHooks(t *testing.T) {
	t.Parallel()

	runner := newRunner(t, `exit 0`)
	inputs := []fantasy.AgentTool{&fakeTool{name: "a"}, &fakeTool{name: "b"}}

	t.Run("top-level agent wraps every tool", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, nil, false)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
	})

	t.Run("post-only runner still wraps", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, nil, runner, false)
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
	})

	t.Run("sub-agent skips the wrap", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, runner, true)
		require.Equal(t, inputs, out, "sub-agent tools should be returned unwrapped")
		for _, tool := range out {
			_, isHooked := tool.(*hookedTool)
			require.False(t, isHooked, "sub-agent tool should not be wrapped")
		}
	})

	t.Run("nil runners skip the wrap for both agent kinds", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, nil, false))
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, nil, true))
	})
}

// captureHook returns a hook command that records its stdin payload to
// payloadPath so tests can assert on what PostToolUse hooks were fed.
func captureHook(payloadPath string) string {
	return fmt.Sprintf("cat > '%s'", payloadPath)
}

func readCapturedPayload(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "hook should have been invoked and captured its payload")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	return payload
}

func TestHookedTool_PostHookFiresAfterInnerTool(t *testing.T) {
	t.Parallel()

	payloadPath := filepath.Join(t.TempDir(), "post.json")
	inner := &fakeTool{name: "write", resp: fantasy.NewTextResponse("wrote file")}
	tool := newHookedTool(inner, nil, newEventRunner(t, hooks.EventPostToolUse, captureHook(payloadPath)))

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-4", Name: "write", Input: `{"file_path":"a.txt"}`})
	require.NoError(t, err)
	require.True(t, inner.called)

	payload := readCapturedPayload(t, payloadPath)
	require.Equal(t, hooks.EventPostToolUse, payload["event"])
	require.Equal(t, "write", payload["tool_name"])
	output, ok := payload["tool_output"].(map[string]any)
	require.True(t, ok, "post payload should carry tool_output")
	require.Equal(t, "wrote file", output["content"])
	require.NotContains(t, output, "is_error")
}

func TestHookedTool_PostHookSeesRawToolOutput(t *testing.T) {
	t.Parallel()

	payloadPath := filepath.Join(t.TempDir(), "post.json")
	pre := newRunner(t, `echo '{"context":"pre-injected"}'`)
	post := newEventRunner(t, hooks.EventPostToolUse, captureHook(payloadPath))
	inner := &fakeTool{name: "write", resp: fantasy.NewTextResponse("raw output")}
	tool := newHookedTool(inner, pre, post)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-5", Name: "write"})
	require.NoError(t, err)

	payload := readCapturedPayload(t, payloadPath)
	output := payload["tool_output"].(map[string]any)
	require.Equal(t, "raw output", output["content"], "post hooks observe the raw tool output, not hook-injected context")

	// Both contexts still surface to the model, pre context before post.
	require.Equal(t, "raw output\npre-injected", resp.Content)
}

func TestHookedTool_PostContextAppendedToResponse(t *testing.T) {
	t.Parallel()

	pre := newRunner(t, `echo '{"context":"from-pre"}'`)
	post := newEventRunner(t, hooks.EventPostToolUse, `echo '{"context":"from-post"}'`)
	inner := &fakeTool{name: "write", resp: fantasy.NewTextResponse("base")}
	tool := newHookedTool(inner, pre, post)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-6", Name: "write"})
	require.NoError(t, err)
	require.Equal(t, "base\nfrom-pre\nfrom-post", resp.Content)
}

func TestHookedTool_PostDecisionIsIgnored(t *testing.T) {
	t.Parallel()

	post := newEventRunner(t, hooks.EventPostToolUse, `echo '{"decision":"deny","reason":"too late"}'`)
	inner := &fakeTool{name: "write", resp: fantasy.NewTextResponse("done")}
	tool := newHookedTool(inner, nil, post)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-7", Name: "write"})
	require.NoError(t, err)
	require.True(t, inner.called)
	require.False(t, resp.IsError, "post hooks cannot veto a completed tool call")
	require.Equal(t, "done", resp.Content)
}

func TestHookedTool_PostHookSkippedOnInnerError(t *testing.T) {
	t.Parallel()

	payloadPath := filepath.Join(t.TempDir(), "post.json")
	post := newEventRunner(t, hooks.EventPostToolUse, captureHook(payloadPath))
	inner := &fakeTool{name: "write", runErr: errors.New("disk full")}
	tool := newHookedTool(inner, nil, post)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-8", Name: "write"})
	require.Error(t, err)
	_, statErr := os.Stat(payloadPath)
	require.ErrorIs(t, statErr, os.ErrNotExist, "failed tool calls must not fire post hooks")
}

func TestHookedTool_PostHookSkippedWhenPreDenies(t *testing.T) {
	t.Parallel()

	payloadPath := filepath.Join(t.TempDir(), "post.json")
	pre := newRunner(t, `echo "blocked" >&2; exit 2`)
	post := newEventRunner(t, hooks.EventPostToolUse, captureHook(payloadPath))
	inner := &fakeTool{name: "bash"}
	tool := newHookedTool(inner, pre, post)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-9", Name: "bash"})
	require.NoError(t, err)
	require.False(t, inner.called)
	_, statErr := os.Stat(payloadPath)
	require.ErrorIs(t, statErr, os.ErrNotExist, "denied tool calls must not fire post hooks")
}

func TestHookedTool_PreOnlyStillWorksWithoutPostRunner(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	tool := newHookedTool(inner, newRunner(t, `exit 0`), nil)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-10", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called)
	require.Equal(t, "ok", resp.Content)
}
