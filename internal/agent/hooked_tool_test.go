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

// newRunner builds a hooks.Runner with a single PreToolUse hook, running
// the config-loader path that validates the config.
func newRunner(t *testing.T, cmd string) *hooks.Runner {
	t.Helper()
	return newRunnerFor(t, map[string][]config.HookConfig{
		hooks.EventPreToolUse: {{Command: cmd}},
	})
}

func newRunnerFor(t *testing.T, cfgHooks map[string][]config.HookConfig) *hooks.Runner {
	t.Helper()
	cfg := &config.Config{Hooks: cfgHooks}
	require.NoError(t, cfg.ValidateHooks())
	return hooks.NewRunner(hooks.Static(cfg.Hooks), t.TempDir(), t.TempDir())
}

func TestHookedTool_AllowStampsHookApproval(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	runner := newRunner(t, `echo '{"decision":"allow"}'`)
	tool := newHookedTool(inner, runner, false)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-1", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called, "inner tool should have run")

	// The inner tool's permission service can now treat call-1 as pre-approved.
	svc := permission.NewPermissionService(t.TempDir(), false, nil, nil)
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
	tool := newHookedTool(inner, runner, false)

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-2", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called)

	// With no hook opinion, a fresh permission request has nothing stamped
	// and must fall through to the normal flow. We verify by checking that
	// the context does not look pre-approved for this call ID: sending a
	// request that no subscriber resolves will block until cancelled.
	svc := permission.NewPermissionService(t.TempDir(), false, nil, nil)
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
	tool := newHookedTool(inner, runner, false)

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
	hookCfgs := map[string][]hooks.HookConfig{
		hooks.EventPreToolUse: {{Command: `exit 0`}},
	}

	t.Run("top-level agent wraps every tool", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, false, hookCfgs)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
	})

	t.Run("sub-agent skips the wrap when no hook opts in", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, true, hookCfgs)
		require.Equal(t, inputs, out, "sub-agent tools should be returned unwrapped")
		for _, tool := range out {
			_, isHooked := tool.(*hookedTool)
			require.False(t, isHooked, "sub-agent tool should not be wrapped")
		}
	})

	t.Run("sub-agent wraps when a hook has IncludeSubAgents=true", func(t *testing.T) {
		t.Parallel()
		optInCfgs := map[string][]hooks.HookConfig{
			hooks.EventPreToolUse: {{Command: `exit 0`, IncludeSubAgents: true}},
		}
		out := wrapToolsWithHooks(inputs, runner, true, optInCfgs)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool when sub-agent opt-in is set", i)
		}
	})

	t.Run("nil runner skips the wrap for both agent kinds", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, false, hookCfgs))
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, true, hookCfgs))
	})
}

func TestHookedTool_PostToolUse(t *testing.T) {
	t.Parallel()

	t.Run("replaces output and keeps pre context", func(t *testing.T) {
		t.Parallel()
		inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("token=sk-secret")}
		runner := newRunnerFor(t, map[string][]config.HookConfig{
			hooks.EventPreToolUse:  {{Command: `echo '{"context":"pre note"}'`}},
			hooks.EventPostToolUse: {{Command: `jq -r .tool_response | grep -q sk-secret || exit 1; echo '{"updated_output":"token=[REDACTED]","context":"post note"}'`}},
		})
		tool := newHookedTool(inner, runner, false)

		resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "bash", Input: `{"command":"env"}`})
		require.NoError(t, err)
		require.Equal(t, "token=[REDACTED]\npre note\npost note", resp.Content)
		require.Contains(t, resp.Metadata, `"hook_count":2`)
		require.Contains(t, resp.Metadata, `"event":"PostToolUse"`)
	})

	t.Run("block feeds reason back and halt stops the turn", func(t *testing.T) {
		t.Parallel()
		inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("committed")}
		runner := newRunnerFor(t, map[string][]config.HookConfig{
			hooks.EventPostToolUse: {{Command: `echo '{"decision":"block","reason":"run the tests first","halt":true}'`}},
		})
		tool := newHookedTool(inner, runner, false)

		resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "bash", Input: `{}`})
		require.NoError(t, err)
		require.True(t, inner.called, "post hooks cannot undo the call")
		require.Contains(t, resp.Content, "Hook feedback: run the tests first")
		require.True(t, resp.StopTurn)
	})

	t.Run("failure fires PostToolUseFailure instead", func(t *testing.T) {
		t.Parallel()
		inner := &fakeTool{name: "bash", resp: fantasy.NewTextErrorResponse("boom")}
		runner := newRunnerFor(t, map[string][]config.HookConfig{
			hooks.EventPostToolUse:        {{Command: `echo '{"updated_output":"should not apply"}'`}},
			hooks.EventPostToolUseFailure: {{Command: `jq -r .error | grep -q boom || exit 1; echo '{"context":"seen failure"}'`}},
		})
		tool := newHookedTool(inner, runner, false)

		resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "c", Name: "bash", Input: `{}`})
		require.NoError(t, err)
		require.True(t, resp.IsError)
		require.Equal(t, "boom\nseen failure", resp.Content)
	})
}

func TestAnyHookIncludesSubAgents(t *testing.T) {
	t.Parallel()

	t.Run("empty config returns false", func(t *testing.T) {
		t.Parallel()
		require.False(t, anyHookIncludesSubAgents(nil))
		require.False(t, anyHookIncludesSubAgents(map[string][]hooks.HookConfig{}))
	})

	t.Run("no hooks with IncludeSubAgents returns false", func(t *testing.T) {
		t.Parallel()
		cfgs := map[string][]hooks.HookConfig{
			hooks.EventPreToolUse: {{Command: `exit 0`}},
		}
		require.False(t, anyHookIncludesSubAgents(cfgs))
	})

	t.Run("one hook with IncludeSubAgents returns true", func(t *testing.T) {
		t.Parallel()
		cfgs := map[string][]hooks.HookConfig{
			hooks.EventPreToolUse: {{Command: `exit 0`, IncludeSubAgents: true}},
		}
		require.True(t, anyHookIncludesSubAgents(cfgs))
	})

	t.Run("multiple hooks, only some opt in", func(t *testing.T) {
		t.Parallel()
		cfgs := map[string][]hooks.HookConfig{
			hooks.EventPreToolUse:  {{Command: `exit 0`}},
			hooks.EventPostToolUse: {{Command: `exit 0`, IncludeSubAgents: true}},
		}
		require.True(t, anyHookIncludesSubAgents(cfgs))
	})

	t.Run("IncludeSubAgents on non-tool event is ignored by anyHookIncludesSubAgents", func(t *testing.T) {
		t.Parallel()
		cfgs := map[string][]hooks.HookConfig{
			hooks.EventTurnStart: {{Command: `exit 0`, IncludeSubAgents: true}},
		}
		// The flag is stored regardless of event type; filtering happens
		// at match time in the runner. This test just checks the helper.
		require.True(t, anyHookIncludesSubAgents(cfgs))
	})
}
