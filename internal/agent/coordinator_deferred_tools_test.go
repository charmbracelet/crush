package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/mailbox"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestActivateDeferredToolsAndBuildToolsIncludesActivatedDeferred(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{
		cfg:                        cfg,
		sessions:                   env.sessions,
		messages:                   env.messages,
		permissions:                env.permissions,
		history:                    env.history,
		filetracker:                *env.filetracker,
		lspManager:                 lsp.NewManager(cfg),
		mailbox:                    mailbox.NewService(),
		activatedDeferredBySession: map[string]map[string]struct{}{},
	}

	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, "session-1")
	activated := coord.activateDeferredTools(ctx, []string{"sourcegraph"})
	require.Equal(t, []string{"sourcegraph"}, activated)

	coder := cfg.Config().Agents[config.AgentCoder]
	toolSet, err := coord.buildTools(ctx, coder, session.CollaborationModeDefault)
	require.NoError(t, err)

	found := false
	for _, tool := range toolSet {
		if tool.Info().Name == "sourcegraph" {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestClearDeferredToolActivationsForSession(t *testing.T) {
	t.Parallel()

	coord := &coordinator{activatedDeferredBySession: map[string]map[string]struct{}{
		"session-1": {"sourcegraph": {}},
	}}

	coord.clearDeferredToolActivationsForSession("session-1")
	require.Empty(t, coord.activatedDeferredBySession)
}
