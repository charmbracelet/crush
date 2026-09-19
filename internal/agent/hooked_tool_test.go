package agent

import (
	"context"
	"errors"
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
	err    error
}

func (f *fakeTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: f.name}
}

func (f *fakeTool) Run(ctx context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
	f.called = true
	f.gotCtx = ctx
	return f.resp, f.err
}

func (f *fakeTool) ProviderOptions() fantasy.ProviderOptions     { return nil }
func (f *fakeTool) SetProviderOptions(_ fantasy.ProviderOptions) {}

// newRunner builds a hooks.Dispatcher from a single PreToolUse HookConfig,
// running the config-loader path that compiles the matcher regex.
func newRunner(t *testing.T, cmd string) *hooks.Dispatcher {
	t.Helper()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventPreToolUse: {{Command: cmd}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())
	return hooks.NewDispatcher(cfg.Hooks, t.TempDir(), t.TempDir())
}

func TestHookedTool_AllowStampsHookApproval(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	runner := newRunner(t, `echo '{"decision":"allow"}'`)
	tool := newHookedTool(inner, runner)

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
	tool := newHookedTool(inner, runner)

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
	tool := newHookedTool(inner, runner)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-3", Name: "bash"})
	require.NoError(t, err)
	require.False(t, inner.called, "denied call must not reach the inner tool")
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "blocked")
}

func TestHookedTool_PostToolUseRunsAfterInnerTool(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	marker := filepath.Join(dir, "order.log")
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventPostToolUse: {{
				Command: `printf 'hook:%s\n' "$CRUSH_TOOL_NAME" >> ` + marker,
			}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("tool output")}
	tool := newHookedTool(inner, hooks.NewDispatcher(cfg.Hooks, dir, dir))

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-9", Name: "view"})
	require.NoError(t, err)
	require.True(t, inner.called)

	got, err := os.ReadFile(marker)
	require.NoError(t, err, "PostToolUse hook should have run")
	require.Equal(t, "hook:view\n", string(got))
	require.Contains(t, resp.Content, "tool output")
}

func TestHookedTool_PostToolUseConfinedToMatchingTool(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	marker := filepath.Join(dir, "order.log")
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventPostToolUse: {{
				Matcher: "^edit$",
				Command: `printf 'edit\n' >> ` + marker,
			}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	tool := newHookedTool(inner, hooks.NewDispatcher(cfg.Hooks, dir, dir))

	_, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-10", Name: "view"})
	require.NoError(t, err)

	_, err = os.Stat(marker)
	require.ErrorIs(t, err, os.ErrNotExist, "matcher-scoped hook must not fire for another tool")
}

func TestHookedTool_PostToolUseDoesNotBlockDeniedCall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	marker := filepath.Join(dir, "order.log")
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventPreToolUse: {{
				Command: `exit 2`,
			}},
			hooks.EventPostToolUse: {{
				Command: `printf 'post\n' >> ` + marker,
			}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())

	inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("ok")}
	tool := newHookedTool(inner, hooks.NewDispatcher(cfg.Hooks, dir, dir))

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-11", Name: "bash"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.False(t, inner.called)
}

func TestHookedTool_PostToolUseSeesFailedTool(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	marker := filepath.Join(dir, "response.txt")
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventPostToolUse: {{
				Command: `printf '%s' "$CRUSH_TOOL_NAME" > ` + marker,
			}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())

	inner := &fakeTool{name: "bash", err: errors.New("boom")}
	tool := newHookedTool(inner, hooks.NewDispatcher(cfg.Hooks, dir, dir))

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-12", Name: "bash"})
	require.Error(t, err, "the tool error must still surface to the caller")

	got, readErr := os.ReadFile(marker)
	require.NoError(t, readErr, "PostToolUse should observe a failed tool call too")
	require.Equal(t, "bash", string(got))
	require.Contains(t, resp.Metadata, `"hook"`,
		"a hook that ran must stay visible in the metadata of a failed call")
}

func TestAppendHookContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		contexts []string
		want     string
	}{
		{"no contexts leaves content alone", "output", nil, "output"},
		{"empty contexts are skipped", "output", []string{"", ""}, "output"},
		{"one context is appended", "output", []string{"note"}, "output\nnote"},
		{"contexts keep their order", "output", []string{"first", "second"}, "output\nfirst\nsecond"},
		{"empty content takes the first context without a separator", "", []string{"note"}, "note"},
		{"empty content skips leading empties", "", []string{"", "note"}, "note"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, appendHookContext(tt.content, tt.contexts...))
		})
	}
}

func TestWrapToolsWithHooks(t *testing.T) {
	t.Parallel()

	runner := newRunner(t, `exit 0`)
	inputs := []fantasy.AgentTool{&fakeTool{name: "a"}, &fakeTool{name: "b"}}

	t.Run("top-level agent wraps every tool", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, false)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
	})

	t.Run("sub-agent skips the wrap", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, true)
		require.Equal(t, inputs, out, "sub-agent tools should be returned unwrapped")
		for _, tool := range out {
			_, isHooked := tool.(*hookedTool)
			require.False(t, isHooked, "sub-agent tool should not be wrapped")
		}
	})

	t.Run("nil runner skips the wrap for both agent kinds", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, false))
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, true))
	})
}
