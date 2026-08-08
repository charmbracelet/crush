package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

// newBuildToolsCoordinator builds a minimal coordinator sufficient to
// exercise buildTools directly, without a full NewCoordinator wiring or any
// network access, mirroring newGateTestCoordinator's minimal-field pattern.
func newBuildToolsCoordinator(cfg *config.ConfigStore, env fakeEnv) *coordinator {
	return &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
	}
}

// TestBuildTools_HookWrapping_TaskAgentBypassesHooks verifies that buildTools
// skips PreToolUse hook interception only for the built-in task agent
// ("task" is a reserved subagent name; Subagent.ToConfigAgent sets ID:
// s.Name for every other subagent). A custom subagent — which now runs with
// tools capped by its dispatching agent rather than always inheriting the
// coder's full tool set — must still fire the user's hooks like any other
// tool caller, since it can carry write tools too.
func TestBuildTools_HookWrapping_TaskAgentBypassesHooks(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()
	cfg.Config().Hooks = map[string][]config.HookConfig{
		hooks.EventPreToolUse: {{Command: "true"}},
	}

	coord := newBuildToolsCoordinator(cfg, env)

	t.Run("custom subagent tools are wrapped", func(t *testing.T) {
		t.Parallel()
		agentCfg := config.Agent{ID: "reviewer", Name: "Reviewer", AllowedTools: []string{"view"}}
		got, err := coord.buildTools(t.Context(), agentCfg, true)
		require.NoError(t, err)
		require.NotEmpty(t, got)
		for _, tool := range got {
			_, ok := tool.(*hookedTool)
			require.True(t, ok, "custom subagent tools must fire PreToolUse hooks")
		}
	})

	t.Run("built-in task agent tools are not wrapped", func(t *testing.T) {
		t.Parallel()
		agentCfg := config.Agent{ID: config.AgentTask, Name: "Task", AllowedTools: []string{"view"}}
		got, err := coord.buildTools(t.Context(), agentCfg, true)
		require.NoError(t, err)
		require.NotEmpty(t, got)
		for _, tool := range got {
			_, ok := tool.(*hookedTool)
			require.False(t, ok, "the built-in task agent must not fire user hooks")
		}
	})
}

// TestBuildTools_MCPResourceToolsRespectAllowedMCP verifies that the
// list_mcp_resources/read_mcp_resource tools are only added for agents with
// no MCP restrictions (AllowedMCP == nil). An agent with a restricted
// AllowedMCP — even an explicitly empty map, meaning "no MCP servers" — must
// not be able to browse resources from every configured MCP server.
func TestBuildTools_MCPResourceToolsRespectAllowedMCP(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()
	cfg.Config().MCP = config.MCPs{
		"test-mcp": {Type: config.MCPStdio, Command: "true"},
	}

	coord := newBuildToolsCoordinator(cfg, env)

	allowedTools := []string{tools.ListMCPResourcesToolName, tools.ReadMCPResourceToolName}

	t.Run("restricted AllowedMCP hides resource tools", func(t *testing.T) {
		t.Parallel()
		agentCfg := config.Agent{
			ID:           "restricted",
			AllowedTools: allowedTools,
			AllowedMCP:   map[string][]string{},
		}
		got, err := coord.buildTools(t.Context(), agentCfg, false)
		require.NoError(t, err)
		for _, tool := range got {
			require.NotEqual(t, tools.ListMCPResourcesToolName, tool.Info().Name)
			require.NotEqual(t, tools.ReadMCPResourceToolName, tool.Info().Name)
		}
	})

	t.Run("unrestricted AllowedMCP keeps resource tools", func(t *testing.T) {
		t.Parallel()
		agentCfg := config.Agent{
			ID:           "unrestricted",
			AllowedTools: allowedTools,
			AllowedMCP:   nil,
		}
		got, err := coord.buildTools(t.Context(), agentCfg, false)
		require.NoError(t, err)
		names := make([]string, 0, len(got))
		for _, tool := range got {
			names = append(names, tool.Info().Name)
		}
		require.Contains(t, names, tools.ListMCPResourcesToolName)
		require.Contains(t, names, tools.ReadMCPResourceToolName)
	})
}
