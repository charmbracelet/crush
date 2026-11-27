package agent

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEffectiveToolNamesByRunMode pins how run mode picks a delegation
// style. A TUI can receive a subagent's report as a later turn, so
// delegation detaches; a headless run exits before one could arrive, so
// delegation blocks instead.
func TestEffectiveToolNamesByRunMode(t *testing.T) {
	t.Parallel()

	agentCfg := config.Agent{
		AllowedTools: []string{
			AgentToolName,
			AgentDispatchToolName,
			AgentSendToolName,
			"question",
			"view",
		},
	}

	tests := []struct {
		name        string
		interactive bool
		isSubAgent  bool
		want        []string
	}{
		{
			name:        "interactive delegates detached",
			interactive: true,
			want:        []string{AgentDispatchToolName, AgentSendToolName, "question", "view"},
		},
		{
			name:        "headless delegates blocking",
			interactive: false,
			want:        []string{AgentToolName, "view"},
		},
		{
			name:        "subagents neither fan out nor ask",
			interactive: true,
			isSubAgent:  true,
			want:        []string{"view"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &coordinator{interactive: tt.interactive}
			assert.Equal(t, tt.want, c.effectiveToolNames(agentCfg, tt.isSubAgent))
		})
	}
}

// TestEffectiveToolNamesKeepsConfigAuthoritative checks the gate only ever
// narrows: a tool the user disabled cannot reappear because of run mode.
func TestEffectiveToolNamesKeepsConfigAuthoritative(t *testing.T) {
	t.Parallel()

	agentCfg := config.Agent{AllowedTools: []string{"view"}}
	for _, interactive := range []bool{true, false} {
		c := &coordinator{interactive: interactive}
		assert.Equal(t, []string{"view"}, c.effectiveToolNames(agentCfg, false))
	}
}

// TestResolveSubAgentAccess covers how a requested access level maps onto a
// subagent variant, including plan mode, which may only delegate work it
// could have done itself.
func TestResolveSubAgentAccess(t *testing.T) {
	t.Parallel()

	read := &mockSessionAgent{}
	write := &mockSessionAgent{}
	both := map[string]SessionAgent{accessRead: read, accessWrite: write}
	readOnly := map[string]SessionAgent{accessRead: read}

	t.Run("empty access defaults to read", func(t *testing.T) {
		t.Parallel()
		agent, refusal := resolveSubAgent(both, "", false)
		require.Empty(t, refusal)
		assert.Same(t, read, agent)
	})

	t.Run("write is available to a writing parent", func(t *testing.T) {
		t.Parallel()
		agent, refusal := resolveSubAgent(both, accessWrite, false)
		require.Empty(t, refusal)
		assert.Same(t, write, agent)
	})

	t.Run("plan mode refuses write with a reason", func(t *testing.T) {
		t.Parallel()
		agent, refusal := resolveSubAgent(readOnly, accessWrite, true)
		assert.Nil(t, agent)
		assert.Contains(t, refusal, "read-only")
	})

	t.Run("plan mode still delegates reads", func(t *testing.T) {
		t.Parallel()
		agent, refusal := resolveSubAgent(readOnly, accessRead, true)
		require.Empty(t, refusal)
		assert.Same(t, read, agent)
	})

	t.Run("nonsense access is rejected", func(t *testing.T) {
		t.Parallel()
		agent, refusal := resolveSubAgent(both, "admin", false)
		assert.Nil(t, agent)
		assert.Contains(t, refusal, `"read" or "write"`)
	})
}

// TestReadOnlyParent documents which parents may only launch read-only
// subagents.
func TestReadOnlyParent(t *testing.T) {
	t.Parallel()

	assert.True(t, readOnlyParent(config.AgentPlan))
	assert.False(t, readOnlyParent(config.AgentCoder))
	assert.False(t, readOnlyParent(""))
}

// TestDispatchToolsNarrowTheSubagent pins the per-dispatch tool restriction:
// a subagent asked for a narrow toolset must not be able to run anything
// else. Without this, a delegated agent that is talked into a destructive
// command has the full toolset to do it with.
func TestDispatchToolsNarrowTheSubagent(t *testing.T) {
	t.Parallel()

	// The parent may do anything; the subagent asks for read-only tools.
	parentCfg := config.Agent{
		AllowedTools: []string{"view", "grep", "glob", "bash", "write", "edit"},
	}

	restricted, err := narrowAgentTools(parentCfg, []string{"view", "grep"})
	require.NoError(t, err)

	c := &coordinator{interactive: false}
	assert.Equal(t, []string{"view", "grep"}, c.effectiveToolNames(restricted, true))

	t.Run("omitting the list keeps the parent toolset", func(t *testing.T) {
		t.Parallel()
		same, err := narrowAgentTools(parentCfg, nil)
		require.NoError(t, err)
		assert.Equal(t, parentCfg.AllowedTools, same.AllowedTools)
	})

	t.Run("naming a tool the parent lacks is refused", func(t *testing.T) {
		t.Parallel()
		_, err := narrowAgentTools(parentCfg, []string{"view", "teleport"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "teleport")
	})

	t.Run("an empty list means no tools at all", func(t *testing.T) {
		t.Parallel()
		none, err := narrowAgentTools(parentCfg, []string{})
		require.NoError(t, err)
		assert.Empty(t, none.AllowedTools)
	})
}

// TestNarrowedTaskAgentKeepsAccessSemantics pins the two invariants a
// restricted dispatch depends on: the base config comes from the right access
// level, and narrowing only ever removes tools.
func TestNarrowedTaskAgentKeepsAccessSemantics(t *testing.T) {
	t.Parallel()

	readBase := config.Agent{AllowedTools: []string{"view", "grep"}}
	writeBase := config.Agent{AllowedTools: []string{"view", "grep", "bash", "write"}}

	t.Run("a write subagent keeps its wider base before narrowing", func(t *testing.T) {
		t.Parallel()
		kept, err := narrowAgentTools(writeBase, []string{"view", "grep", "bash"})
		require.NoError(t, err)
		assert.Contains(t, kept.AllowedTools, "bash",
			"a write subagent must not lose write-level tools it asked to keep")
	})

	t.Run("a read subagent cannot ask for a write-only tool", func(t *testing.T) {
		t.Parallel()
		_, err := narrowAgentTools(readBase, []string{"bash"})
		require.Error(t, err, "read base has no bash, so asking for it must be refused")
	})

	t.Run("narrowing never adds a tool", func(t *testing.T) {
		t.Parallel()
		for _, base := range []config.Agent{readBase, writeBase} {
			out, err := narrowAgentTools(base, []string{"view"})
			require.NoError(t, err)
			assert.LessOrEqual(t, len(out.AllowedTools), len(base.AllowedTools))
		}
	})
}

// testNarrowingCoordinator wires the least a task subagent needs to build:
// config, sessions, messages, permissions, history and a file tracker. In
// particular it has no lspManager or shellOwner, which is the point - a
// restricted subagent must not require them.
func testNarrowingCoordinator(t *testing.T) *coordinator {
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

	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	return &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
	}
}

func narrowedAgentToolNames(t *testing.T, agent SessionAgent) []string {
	t.Helper()

	sa, ok := agent.(*sessionAgent)
	require.True(t, ok, "narrowTaskAgent returns the session agent implementation")

	names := make([]string, 0, sa.tools.Len())
	for _, tool := range sa.tools.Copy() {
		names = append(names, tool.Info().Name)
	}
	sort.Strings(names)
	return names
}

// TestNarrowTaskAgentBuildsOnlyRequestedTools is the integration counterpart
// of TestDispatchToolsNarrowTheSubagent: narrowing has to survive all the way
// into the agent that will run, not just into a config struct.
func TestNarrowTaskAgentBuildsOnlyRequestedTools(t *testing.T) {
	t.Parallel()

	c := testNarrowingCoordinator(t)

	agent, err := c.narrowTaskAgent(t.Context(), accessRead, []string{"view", "grep"})
	require.NoError(t, err)

	assert.Equal(t, []string{"grep", "view"}, narrowedAgentToolNames(t, agent))
}

// TestNarrowTaskAgentKeepsTheAccessCeiling pins that the access level is what
// decides whether a tool exists at all: the read task agent may run bash but
// must not write, and the write variant is allowed what the coder has.
func TestNarrowTaskAgentKeepsTheAccessCeiling(t *testing.T) {
	t.Parallel()

	c := testNarrowingCoordinator(t)

	_, err := c.narrowTaskAgent(t.Context(), accessRead, []string{"write"})
	require.Error(t, err, "a read subagent must not be narrowed onto a write-only tool")
	assert.Contains(t, err.Error(), `"write"`)

	agent, err := c.narrowTaskAgent(t.Context(), accessWrite, []string{"write"})
	require.NoError(t, err)

	assert.Equal(t, []string{"write"}, narrowedAgentToolNames(t, agent))
}
