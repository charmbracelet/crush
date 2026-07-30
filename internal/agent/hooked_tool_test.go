package agent

import (
	"context"
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
}

func (f *fakeTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: f.name}
}

func (f *fakeTool) Run(ctx context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
	f.called = true
	f.gotCtx = ctx
	return f.resp, nil
}

func (f *fakeTool) ProviderOptions() fantasy.ProviderOptions     { return nil }
func (f *fakeTool) SetProviderOptions(_ fantasy.ProviderOptions) {}

// newRunner builds a hooks.Runner from a single HookConfig, running the
// config-loader path that compiles the matcher regex.
func newRunner(t *testing.T, cmd string) *hooks.Runner {
	t.Helper()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventPreToolUse: {{Command: cmd}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())
	return hooks.NewRunner(cfg.Hooks[hooks.EventPreToolUse], t.TempDir(), t.TempDir())
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

	t.Run("sub-agent skips the wrap", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, nil, true)
		require.Equal(t, inputs, out, "sub-agent tools should be returned unwrapped")
		for _, tool := range out {
			_, isHooked := tool.(*hookedTool)
			require.False(t, isHooked, "sub-agent tool should not be wrapped")
		}
	})

	t.Run("nil runner skips the wrap for both agent kinds", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, nil, false))
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, nil, true))
	})
}

func newPostRunner(t *testing.T, cmd string) *hooks.Runner {
	t.Helper()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventPostToolUse: {{Command: cmd}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())
	return hooks.NewRunner(cfg.Hooks[hooks.EventPostToolUse], t.TempDir(), t.TempDir())
}

func TestHookedTool_PostToolUseContext(t *testing.T) {
	t.Parallel()
	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	post := newPostRunner(t, `echo '{"context":"post-ctx"}'`)
	tool := newHookedTool(inner, nil, post)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c1", Name: "view", Input: `{}`})
	require.NoError(t, err)
	require.True(t, inner.called)
	require.Contains(t, resp.Content, "ok")
	require.Contains(t, resp.Content, "post-ctx")
}

func TestHookedTool_PostToolUseHalt(t *testing.T) {
	t.Parallel()
	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	post := newPostRunner(t, `echo '{"halt":true,"reason":"stop"}'`)
	tool := newHookedTool(inner, nil, post)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c1", Name: "view", Input: `{}`})
	require.NoError(t, err)
	require.True(t, resp.StopTurn)
}

func TestHookedTool_PostToolUseDenyNudge(t *testing.T) {
	t.Parallel()
	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	post := newPostRunner(t, `echo '{"decision":"deny","reason":"careful"}'`)
	tool := newHookedTool(inner, nil, post)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c1", Name: "view", Input: `{}`})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "[post-hook] careful")
}

func TestHookedTool_PreDenySkipsPost(t *testing.T) {
	t.Parallel()
	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	pre := newRunner(t, `echo '{"decision":"deny","reason":"no"}'`)
	post := newPostRunner(t, `echo '{"context":"should-not-run"}'`)
	tool := newHookedTool(inner, pre, post)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c1", Name: "view", Input: `{}`})
	require.NoError(t, err)
	require.False(t, inner.called)
	require.NotContains(t, resp.Content, "should-not-run")
}

func TestWrapToolsWithHooks_PostOnly(t *testing.T) {
	t.Parallel()
	inner := &fakeTool{name: "view"}
	post := newPostRunner(t, `echo '{}'`)
	out := wrapToolsWithHooks([]fantasy.AgentTool{inner}, nil, post, false)
	require.Len(t, out, 1)
	_, ok := out[0].(*hookedTool)
	require.True(t, ok)
}
