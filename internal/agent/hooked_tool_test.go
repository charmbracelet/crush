package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

func TestUnsafeCrushConfigTextEdit(t *testing.T) {
	t.Parallel()

	require.True(t, unsafeCrushConfigTextEdit(fantasy.ToolCall{Name: tools.EditToolName, Input: `{"file_path":"C:\\Users\\me\\AppData\\Local\\crush\\crush.json"}`}))
	require.True(t, unsafeCrushConfigTextEdit(fantasy.ToolCall{Name: tools.MultiEditToolName, Input: `{"file_path":".crush.json"}`}))
	require.False(t, unsafeCrushConfigTextEdit(fantasy.ToolCall{Name: tools.EditToolName, Input: `{"file_path":"main.go"}`}))
}

// fakeTool records the context it was invoked with so tests can assert on
// values stamped onto it by the hookedTool decorator.
type fakeTool struct {
	name   string
	called bool
	calls  int
	gotCtx context.Context
	resp   fantasy.ToolResponse
	err    error
}

func (f *fakeTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: f.name}
}

func (f *fakeTool) Run(ctx context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
	f.called = true
	f.calls++
	f.gotCtx = ctx
	return f.resp, f.err
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

func sessionContext(sessionID string) context.Context {
	return context.WithValue(context.Background(), tools.SessionIDContextKey, sessionID)
}

func TestHookedTool_AllowStampsHookApproval(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	runner := newRunner(t, `echo '{"decision":"allow"}'`)
	tool := newHookedTool(inner, runner, nil, nil, nil)

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
	tool := newHookedTool(inner, runner, nil, nil, nil)

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
	tool := newHookedTool(inner, runner, nil, nil, nil)

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
		out := wrapToolsWithHooks(inputs, runner, nil, nil, false)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
	})

	t.Run("sub-agent skips the wrap", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, runner, nil, nil, true)
		require.Equal(t, inputs, out, "sub-agent tools should be returned unwrapped")
		for _, tool := range out {
			_, isHooked := tool.(*hookedTool)
			require.False(t, isHooked, "sub-agent tool should not be wrapped")
		}
	})

	t.Run("nil runner still wraps top-level tools for recovery ledger", func(t *testing.T) {
		t.Parallel()
		out := wrapToolsWithHooks(inputs, nil, nil, nil, false)
		require.Len(t, out, len(inputs))
		for i, tool := range out {
			_, ok := tool.(*hookedTool)
			require.Truef(t, ok, "tool %d should be a *hookedTool", i)
		}
		require.Equal(t, inputs, wrapToolsWithHooks(inputs, nil, nil, nil, true))
	})
}

func TestHookedTool_PostToolUseAppendsContext(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	postRunner := newRunner(t, `echo '{"context":"reviewed by post hook"}'`)
	tool := newHookedTool(inner, nil, postRunner, nil, nil)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-4", Name: "view", Input: `{}`})
	require.NoError(t, err)
	require.True(t, inner.called)
	require.Contains(t, resp.Content, "ok")
	require.Contains(t, resp.Content, "reviewed by post hook")
}

func TestHookedTool_PostToolUseBlockReplacesResult(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash", resp: fantasy.NewTextResponse("looks fine")}
	postRunner := newRunner(t, `echo '{"decision":"block","reason":"inspect help before retrying"}'`)
	tool := newHookedTool(inner, nil, postRunner, nil, nil)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{ID: "call-5", Name: "bash", Input: `{"command":"bad --flag"}`})
	require.NoError(t, err)
	require.True(t, inner.called)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "inspect help before retrying")
	require.NotContains(t, resp.Content, "looks fine")
}

func TestHookedTool_PostToolUseFailureUsesFailureEvent(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash", resp: fantasy.NewTextErrorResponse("unknown option")}
	postRunner := newRunner(t, `echo '{"context":"regular post hook"}'`)
	postFailureRunner := newRunner(t, `echo '{"decision":"block","reason":"failure hook recovery"}'`)
	tool := newHookedTool(inner, nil, postRunner, postFailureRunner, newToolFailureLedger())

	resp, err := tool.Run(sessionContext("s-failure"), fantasy.ToolCall{ID: "call-6", Name: "bash", Input: `{"command":"pm2 reload app --env-file x"}`})
	require.NoError(t, err)
	require.True(t, inner.called)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "failure hook recovery")
	require.NotContains(t, resp.Content, "regular post hook")
}

func TestHookedTool_BlocksRepeatedFailedInput(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "bash", resp: fantasy.NewTextErrorResponse("unknown option --env-file")}
	tool := newHookedTool(inner, nil, nil, nil, newToolFailureLedger())
	ctx := sessionContext("s-repeat")
	call := fantasy.ToolCall{ID: "call-7", Name: "bash", Input: `{"command":"pm2 reload app --env-file /opt/.env"}`}

	first, err := tool.Run(ctx, call)
	require.NoError(t, err)
	require.True(t, first.IsError)
	require.Equal(t, 1, inner.calls)

	second, err := tool.Run(ctx, call)
	require.NoError(t, err)
	require.True(t, second.IsError)
	require.Contains(t, second.Content, "Repeated failed tool call blocked")
	require.Equal(t, 1, inner.calls, "identical failed retry must not reach the inner tool")
}

func TestHookedTool_BlocksAgentUntilRootGrounded(t *testing.T) {
	t.Parallel()

	ledger := newToolFailureLedger()
	ctx := sessionContext("s-ground")
	agentInner := &fakeTool{name: AgentToolName, resp: fantasy.NewTextResponse("delegated")}
	agentTool := newHookedTool(agentInner, nil, nil, nil, ledger)

	blocked, err := agentTool.Run(ctx, fantasy.ToolCall{ID: "call-8", Name: AgentToolName, Input: `{"prompt":"fix it"}`})
	require.NoError(t, err)
	require.True(t, blocked.IsError)
	require.Contains(t, blocked.Content, "Sub-agent call blocked")
	require.Equal(t, 0, agentInner.calls)

	groundInner := &fakeTool{name: "ls", resp: fantasy.NewTextResponse("package.json")}
	groundTool := newHookedTool(groundInner, nil, nil, nil, ledger)
	grounded, err := groundTool.Run(ctx, fantasy.ToolCall{ID: "call-9", Name: "ls", Input: `{"path":"."}`})
	require.NoError(t, err)
	require.False(t, grounded.IsError)
	require.Equal(t, 1, groundInner.calls)

	allowed, err := agentTool.Run(ctx, fantasy.ToolCall{ID: "call-10", Name: AgentToolName, Input: `{"prompt":"fix it"}`})
	require.NoError(t, err)
	require.False(t, allowed.IsError)
	require.Contains(t, allowed.Content, "delegated")
	require.Equal(t, 1, agentInner.calls)
}
