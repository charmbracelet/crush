package session

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func TestEstimatedUsageStateSurvivesFetchModifySave(t *testing.T) {
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)

	sessions := NewService(db.New(conn), conn)

	created, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	created.PromptTokens = 100
	created.CompletionTokens = 50
	created.EstimatedUsage = true

	saved, err := sessions.Save(t.Context(), created)
	require.NoError(t, err)
	require.True(t, saved.EstimatedUsage)

	fetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.True(t, fetched.EstimatedUsage)

	fetched.Todos = []Todo{{
		Content:    "Check estimate state",
		Status:     TodoStatusInProgress,
		ActiveForm: "Checking estimate state",
	}}

	updated, err := sessions.Save(t.Context(), fetched)
	require.NoError(t, err)
	require.True(t, updated.EstimatedUsage)

	refetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.True(t, refetched.EstimatedUsage)
}

func TestMCPServerDisabledRoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)

	sessions := NewService(db.New(conn), conn)

	disabled, err := sessions.MCPDisabledServers(t.Context())
	require.NoError(t, err)
	require.Empty(t, disabled, "a new repository must default to the config")

	require.NoError(t, sessions.SetMCPServerDisabled(t.Context(), "docker", true))
	require.NoError(t, sessions.SetMCPServerDisabled(t.Context(), "serena", true))
	require.NoError(t, sessions.SetMCPServerDisabled(t.Context(), "docker", true), "disabling twice must be idempotent")

	disabled, err = sessions.MCPDisabledServers(t.Context())
	require.NoError(t, err)
	require.Equal(t, []string{"docker", "serena"}, disabled)

	require.NoError(t, sessions.SetMCPServerDisabled(t.Context(), "docker", false))
	disabled, err = sessions.MCPDisabledServers(t.Context())
	require.NoError(t, err)
	require.Equal(t, []string{"serena"}, disabled)

	// Enabling records an enabled override so a config-disabled server
	// stays enabled across restarts; disabling removes it again.
	enabled, err := sessions.MCPServersEnabled(t.Context())
	require.NoError(t, err)
	require.Equal(t, []string{"docker"}, enabled)

	require.NoError(t, sessions.SetMCPServerDisabled(t.Context(), "docker", true))
	enabled, err = sessions.MCPServersEnabled(t.Context())
	require.NoError(t, err)
	require.Empty(t, enabled)
}

func TestEstimatedUsageStateCanBeClearedByExplicitSave(t *testing.T) {
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)

	sessions := NewService(db.New(conn), conn)

	created, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	created.PromptTokens = 100
	created.CompletionTokens = 50
	created.EstimatedUsage = true

	saved, err := sessions.Save(t.Context(), created)
	require.NoError(t, err)
	require.True(t, saved.EstimatedUsage)

	saved.EstimatedUsage = false
	updated, err := sessions.Save(t.Context(), saved)
	require.NoError(t, err)
	require.False(t, updated.EstimatedUsage)

	refetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.False(t, refetched.EstimatedUsage)
}
