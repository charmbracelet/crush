package agent

import (
	"context"
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

// TestBuildToolsWithOnlyReadToolsNeedsNoServices pins the reason to filter
// before constructing: a subagent allowed nothing but glob and grep should
// build with a coordinator that has no lspManager, permissions, history or
// filetracker. Today every constructor runs regardless, so this fails with a
// nil dereference; after the filter moves earlier, it passes.
func TestBuildToolsWithOnlyReadToolsNeedsNoServices(t *testing.T) {
	t.Parallel()

	agent := config.Agent{AllowedTools: []string{"glob", "grep"}}
	c := &coordinator{interactive: false, cfg: testConfigStore()}

	var built []string
	require.NotPanics(t, func() {
		builtTools, err := c.buildTools(context.Background(), agent, true, nil)
		require.NoError(t, err)
		for _, tool := range builtTools {
			built = append(built, tool.Info().Name)
		}
	})

	// Nothing outside the allowed list may be attached, and the LSP tools that
	// auto_lsp would otherwise construct must not appear either.
	assert.ElementsMatch(t, []string{"glob", "grep"}, built)
}
