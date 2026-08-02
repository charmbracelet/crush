package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

// recordingAgent captures the prompt passed to Run so production call
// sites in coordinator.run can be asserted without a live model.
type recordingAgent struct {
	lastPrompt string
	model      Model
}

func (a *recordingAgent) Run(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	a.lastPrompt = call.Prompt
	return &fantasy.AgentResult{}, nil
}
func (a *recordingAgent) BeginAccepted(string) *AcceptedRun { return nil }
func (a *recordingAgent) SetModels(large, _ Model)          { a.model = large }
func (a *recordingAgent) SetTools([]fantasy.AgentTool)      {}
func (a *recordingAgent) SetSystemPrompt(string)            {}
func (a *recordingAgent) Cancel(string)                     {}
func (a *recordingAgent) CancelAll()                        {}
func (a *recordingAgent) IsSessionBusy(string) bool         { return false }
func (a *recordingAgent) IsBusy() bool                      { return false }
func (a *recordingAgent) QueuedPrompts(string) int          { return 0 }
func (a *recordingAgent) QueuedPromptsList(string) []string { return nil }
func (a *recordingAgent) ClearQueue(string)                 {}
func (a *recordingAgent) Summarize(context.Context, string, fantasy.ProviderOptions, func(context.Context, *fantasy.ProviderError) error) error {
	return nil
}
func (a *recordingAgent) Model() Model                                  { return a.model }
func (a *recordingAgent) GenerateTitle(context.Context, string, string) {}
func (a *recordingAgent) SideQuestion(context.Context, string, string, []SideQuestionExchange) (SideQuestionResult, error) {
	return SideQuestionResult{}, nil
}

func hookEventsCoordinator(t *testing.T) (*coordinator, *recordingAgent) {
	t.Helper()
	env := testEnv(t)
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()
	require.NoError(t, cfg.Config().ValidateHooks())

	// Close any package-level MCP init gate left open by another test
	// (TestBuildAgentReadinessSurvivesCallerCancellation calls ArmInit
	// without completing init). Without this, run()'s WaitForInit hangs
	// for the package timeout when tests share the process.
	mcp.Initialize(t.Context(), env.permissions, cfg)

	rec := &recordingAgent{
		model: Model{
			Model:      &finishStreamModel{text: "x"},
			CatwalkCfg: catwalk.Model{ContextWindow: 8192, DefaultMaxTokens: 128},
			ModelCfg:   config.SelectedModel{Provider: "mock", Model: "mock-model"},
		},
	}
	coord := &coordinator{
		cfg:          cfg,
		sessions:     env.sessions,
		messages:     env.messages,
		permissions:  env.permissions,
		history:      env.history,
		filetracker:  *env.filetracker,
		currentAgent: rec,
	}
	return coord, rec
}

// Deleting the sole production UserPromptSubmit call in coordinator.run
// must fail these tests. The helper alone is not enough — that is the
// call-site half of the skill sec5 proof.
func TestUserPromptSubmitDenyBlocksRun(t *testing.T) {
	// Not parallel: shares package MCP init state with the readiness test.
	coord, rec := hookEventsCoordinator(t)
	cmd := `echo '{"decision":"deny","reason":"nope"}'`
	coord.cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventUserPromptSubmit: {{Command: cmd}},
	}
	require.NoError(t, coord.cfg.Config().ValidateHooks())

	_, err := coord.run(t.Context(), nil, "sess-1", "do a thing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "prompt blocked by hook")
	require.Contains(t, err.Error(), "nope")
	require.Empty(t, rec.lastPrompt, "deny must not reach the agent")
}

func TestUserPromptSubmitUpdatedPromptReachesAgent(t *testing.T) {
	coord, rec := hookEventsCoordinator(t)
	cmd := `echo '{"updated_prompt":"rewritten by hook"}'`
	coord.cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventUserPromptSubmit: {{Command: cmd}},
	}
	require.NoError(t, coord.cfg.Config().ValidateHooks())

	_, err := coord.run(t.Context(), nil, "sess-1", "original prompt")
	require.NoError(t, err)
	require.Equal(t, "rewritten by hook", rec.lastPrompt)
}

func TestApplyUserPromptSubmitHooksDeny(t *testing.T) {
	t.Parallel()
	coord, _ := hookEventsCoordinator(t)
	coord.cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventUserPromptSubmit: {{Command: `echo '{"decision":"deny","reason":"nope"}'`}},
	}
	require.NoError(t, coord.cfg.Config().ValidateHooks())
	_, err := coord.applyUserPromptSubmitHooks(t.Context(), "s", "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nope")
}

// buildTools is the only production path that constructs postRunner and
// hands it to wrapToolsWithHooks. Unit tests that build hookedTool by hand
// stay green when that wiring is deleted.
func TestBuildToolsWiresPostToolUseHooks(t *testing.T) {
	t.Parallel()
	marker := filepath.Join(t.TempDir(), "post-fired")
	cmd := fmt.Sprintf("touch %s", quoteShellPath(marker))
	coord, _ := hookEventsCoordinator(t)
	coord.cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPostToolUse: {{Command: cmd}},
	}
	require.NoError(t, coord.cfg.Config().ValidateHooks())

	agentCfg := coord.cfg.Config().Agents[config.AgentCoder]
	agentCfg.AllowedTools = []string{tools.ViewToolName}

	got, err := coord.buildTools(t.Context(), agentCfg, false)
	require.NoError(t, err)
	require.NotEmpty(t, got)

	var ht *hookedTool
	for _, tl := range got {
		if h, ok := tl.(*hookedTool); ok {
			ht = h
			break
		}
	}
	require.NotNil(t, ht, "top-level tools must be wrapped when PostToolUse is configured")
	require.NotNil(t, ht.post, "post runner must be wired from config")

	inner := &fakeTool{name: "view", resp: fantasy.NewTextResponse("ok")}
	wrapped := newHookedTool(inner, nil, ht.post)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, "sess-post")
	resp, err := wrapped.Run(ctx, fantasy.ToolCall{Name: "view", Input: `{"path":"x"}`})
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	_, statErr := os.Stat(marker)
	require.NoError(t, statErr, "PostToolUse must fire through the buildTools-wired runner")
}

func TestBuildToolsSkipsHooksForSubAgents(t *testing.T) {
	t.Parallel()
	coord, _ := hookEventsCoordinator(t)
	coord.cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPostToolUse: {{Command: "true"}},
		hooks.EventPreToolUse:  {{Command: "true"}},
	}
	require.NoError(t, coord.cfg.Config().ValidateHooks())
	agentCfg := coord.cfg.Config().Agents[config.AgentCoder]
	got, err := coord.buildTools(t.Context(), agentCfg, true)
	require.NoError(t, err)
	for _, tl := range got {
		_, isHooked := tl.(*hookedTool)
		require.False(t, isHooked, "sub-agent tools must not be hook-wrapped")
	}
}
