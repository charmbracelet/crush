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

// newGateTestCoordinator builds a minimal coordinator against a hermetic
// config: one openai-typed provider pointed at a closed port, with large and
// small models selected so model resolution and the system-prompt build both
// succeed without any network access.
func newGateTestCoordinator(t *testing.T) *coordinator {
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

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
		agents:      make(map[string]SessionAgent),
	}

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]

	agent, ready, err := coord.buildAgent(context.Background(), p, agentCfg, false)
	require.NoError(t, err)
	coord.setActiveAgent(config.AgentCoder, agent, ready)

	return coord
}

// TestRunWaitsForMCPOnlyWhenNonInteractive pins the split behavior for
// in-flight MCP initialization.
//
// MCP servers connect asynchronously. Interactive runs must not wait for them:
// blocking the send path meant a slow stdio server (e.g. Python via uv) froze
// the TUI for the length of its connect timeout, most visibly on the first
// message of a session. Tools from late servers simply miss that run's palette
// and show up on the next one.
//
// Non-interactive runs (`crush run`, both local and client/server) get a single
// shot at the palette, so they still wait for initialization to settle.
//
// The split is per run, not per coordinator: one workspace on the shared
// server serves an attached TUI and headless prompts at the same time.
func TestRunWaitsForMCPOnlyWhenNonInteractive(t *testing.T) {
	t.Run("non-interactive waits", func(t *testing.T) {
		coord := newGateTestCoordinator(t)

		// Arm the gate and never complete initialization, standing in for an
		// MCP server that is still connecting.
		mcp.ArmInit()
		t.Cleanup(mcp.DisarmInit)

		ctx, cancel := context.WithTimeout(WithNonInteractive(context.Background()), 200*time.Millisecond)
		defer cancel()

		_, err := coord.run(ctx, nil, "test-session", "hello")
		require.ErrorContains(t, err, "MCP initialization",
			"non-interactive run must block on MCP initialization")
	})

	t.Run("interactive does not wait", func(t *testing.T) {
		coord := newGateTestCoordinator(t)

		mcp.ArmInit()
		t.Cleanup(mcp.DisarmInit)

		done := make(chan error, 1)
		go func() {
			_, err := coord.run(context.Background(), nil, "test-session", "hello")
			done <- err
		}()

		select {
		case err := <-done:
			// The run fails for unrelated reasons (no such session, closed
			// provider port); all that matters is that it got past the gate
			// instead of parking on MCP initialization.
			if err != nil {
				require.NotContains(t, err.Error(), "MCP initialization",
					"interactive run must not block on MCP initialization")
			}
		case <-time.After(10 * time.Second):
			t.Fatal("interactive run blocked; it must not wait for MCP initialization")
		}
	})
}
