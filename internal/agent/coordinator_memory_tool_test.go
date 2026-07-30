package agent

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// newMemoryToolCoordinator builds a coordinator over a minimal hermetic
// config. disableMemory is applied before the config is ever handed to
// buildTools: buildTools spawns agent-readiness goroutines that read
// Options in prompt.promptData, so mutating a config that has already been
// built against is a genuine data race, not just untidy.
func newMemoryToolCoordinator(t *testing.T, disableMemory bool) (*coordinator, config.Agent) {
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
	cfg.Config().Options.DisableMemory = disableMemory
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
	}
	return coord, cfg.Config().Agents[config.AgentCoder]
}

func toolNames(ts []fantasy.AgentTool) []string {
	names := make([]string, 0, len(ts))
	for _, tl := range ts {
		names = append(names, tl.Info().Name)
	}
	return names
}

// TestBuildToolsRegistersMemoryWrite pins the memory_write registration in
// coordinator.buildTools.
//
// Regression test for a reachability gap rather than a past misbehaviour: the
// cassette harness in common_test.go hand-builds its own tool slice and never
// calls buildTools, and config's load_test only checks the allToolNames
// catalogue. Deleting the sole production registration line therefore left
// every test in ./internal/agent/... and ./internal/config/... green, so
// nothing proved the tool was reachable by the coder agent at all.
func TestBuildToolsRegistersMemoryWrite(t *testing.T) {
	coord, agentCfg := newMemoryToolCoordinator(t, false)
	got, err := coord.buildTools(context.Background(), agentCfg, false)
	require.NoError(t, err)
	require.Contains(t, toolNames(got), tools.MemoryWriteToolName,
		"the coder agent must actually be given the memory_write tool")
}

// TestBuildToolsOmitsMemoryWriteForSubAgents keeps sub-agents read-only with
// respect to the project memory store.
func TestBuildToolsOmitsMemoryWriteForSubAgents(t *testing.T) {
	coord, agentCfg := newMemoryToolCoordinator(t, false)
	got, err := coord.buildTools(context.Background(), agentCfg, true)
	require.NoError(t, err)
	require.False(t, slices.Contains(toolNames(got), tools.MemoryWriteToolName),
		"sub-agents must not be given memory_write")
}

// TestBuildToolsOmitsMemoryWriteWhenDisabled proves disable_memory actually
// removes the tool rather than only hiding the prompt section.
func TestBuildToolsOmitsMemoryWriteWhenDisabled(t *testing.T) {
	coord, agentCfg := newMemoryToolCoordinator(t, true)
	got, err := coord.buildTools(context.Background(), agentCfg, false)
	require.NoError(t, err)
	require.False(t, slices.Contains(toolNames(got), tools.MemoryWriteToolName),
		"disable_memory must remove the memory_write tool")
}
