package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestBuildAgentReadinessSurvivesCallerCancellation is a regression test for
// the CRUSH_CLIENT_SERVER=1 "new session hangs" bug.
//
// buildAgent starts readiness goroutines that run mcp.WaitForInit before
// building the tool list. Several server entry points build an agent from a
// short-lived HTTP request context — the InitAgent/UpdateAgent handlers, and
// the sub-agent build reached through UpdateModels -> buildTools -> agentTool.
// When a slow MCP server kept initialization in flight, that request context
// was canceled the moment the handler returned; WaitForInit then observed the
// cancellation, the readyWg errgroup recorded context.Canceled, and every
// later coordinator.run failed at readyWg.Wait() before emitting anything —
// the session hung with no visible LLM response.
//
// The fix detaches the readiness work from the caller context via
// context.WithoutCancel, so canceling the context that triggered the build no
// longer poisons readyWg. Here we arm MCP init so WaitForInit blocks, build an
// agent with a cancelable context, cancel it, and require that readyWg keeps
// waiting for init instead of failing with context.Canceled.
func TestBuildAgentReadinessSurvivesCallerCancellation(t *testing.T) {
	env := testEnv(t)

	// Minimal hermetic config: one openai-typed provider with selected large
	// and small models so buildAgentModels and the system-prompt build both
	// succeed. No MCP servers are configured, so initialization would complete
	// instantly if we let it — we deliberately do not, so WaitForInit stays
	// blocked for the duration of the assertion.
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

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
	}

	// Arm the MCP init gate so buildAgent's readiness goroutine blocks in
	// WaitForInit. We never complete init, so the goroutine stays parked; the
	// agent package's TestMain does not enforce goleak and no other test in it
	// builds a coordinator, so the parked goroutine is harmless.
	mcp.ArmInit()

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]

	ctx, cancel := context.WithCancel(context.Background())
	_, err = coord.buildAgent(ctx, p, agentCfg, false)
	require.NoError(t, err)

	// The caller goes away, mirroring an HTTP handler returning and canceling
	// its request context while MCP init is still in flight.
	cancel()

	done := make(chan error, 1)
	go func() { done <- coord.readyWg.Wait() }()

	select {
	case err := <-done:
		// readyWg finished early. context.Canceled is the regression: the
		// caller's cancellation leaked into the readiness work and poisoned the
		// errgroup. Any other early return means this minimal setup failed to
		// build, which the NoError check surfaces distinctly.
		require.NotErrorIs(t, err, context.Canceled,
			"readyWg was poisoned by caller cancellation (client/server new-session hang regression)")
		require.NoError(t, err, "unexpected buildAgent readiness error")
	case <-time.After(250 * time.Millisecond):
		// readyWg is still waiting on MCP init despite the canceled caller
		// context. This is the fixed behavior.
	}
}
