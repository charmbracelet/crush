package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// The plan agent promises no file changes, so it must not be able to steer
// or reopen background subagents that coder mode started with write tools.
func TestBuildTools_PlanAgentBackgroundTools(t *testing.T) {
	t.Parallel()

	cfg := config.NewTestStoreWithWorkingDir(&config.Config{Options: &config.Options{Attribution: &config.Attribution{}, DataDirectory: t.TempDir()}}, t.TempDir())
	cfg.SetupAgents()
	coder := cfg.Config().Agents[config.AgentCoder]
	plan := cfg.Config().Agents[config.AgentPlan]

	interactive := &coordinator{cfg: cfg, interactive: true}
	headless := &coordinator{cfg: cfg}

	t.Run("interactive coder keeps dispatch, send and stop", func(t *testing.T) {
		t.Parallel()
		names := buildToolNames(t, interactive, coder, false)
		require.Contains(t, names, AgentToolName)
		require.Contains(t, names, AgentSendToolName)
		require.Contains(t, names, AgentStopToolName)
	})

	t.Run("interactive plan keeps read-only dispatch but not send or stop", func(t *testing.T) {
		t.Parallel()
		names := buildToolNames(t, interactive, plan, false)
		require.Contains(t, names, AgentToolName)
		require.NotContains(t, names, AgentSendToolName)
		require.NotContains(t, names, AgentStopToolName)
	})

	t.Run("headless plan gets neither", func(t *testing.T) {
		t.Parallel()
		names := buildToolNames(t, headless, plan, false)
		require.NotContains(t, names, AgentSendToolName)
		require.NotContains(t, names, AgentStopToolName)
	})

	t.Run("subagents get neither", func(t *testing.T) {
		t.Parallel()
		for _, agent := range []config.Agent{coder, plan} {
			names := buildToolNames(t, interactive, agent, true)
			require.NotContains(t, names, AgentSendToolName, agent.ID)
			require.NotContains(t, names, AgentStopToolName, agent.ID)
		}
	})
}
