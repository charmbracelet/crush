package session

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

func TestRenamePreservesSessionActivity(t *testing.T) {
	for _, name := range []string{"fresh", "upgrade"} {
		t.Run(name, func(t *testing.T) {
			dataDir := t.TempDir()
			t.Cleanup(func() { require.NoError(t, db.Release(dataDir)) })

			conn, err := db.Connect(t.Context(), dataDir)
			require.NoError(t, err)
			if name == "upgrade" {
				require.NoError(t, goose.DownTo(conn, "migrations", 20261109000000))
			}

			// Seed persisted activity times without depending on the wall clock.
			_, err = conn.ExecContext(t.Context(), `
				INSERT INTO sessions (id, title, prompt_tokens, cost, created_at, updated_at)
				VALUES ('older', 'Old title', 100, 0.25, 1, 100),
				       ('latest', 'Latest conversation', 200, 0.50, 2, 200)
			`)
			require.NoError(t, err)
			require.NoError(t, db.Release(dataDir))
			conn, err = db.Connect(t.Context(), dataDir)
			require.NoError(t, err)
			sessions := NewService(db.New(conn), conn)

			require.NoError(t, sessions.Rename(t.Context(), "older", "New title"))

			renamed, err := sessions.Get(t.Context(), "older")
			require.NoError(t, err)
			require.Equal(t, "New title", renamed.Title)
			require.Equal(t, int64(100), renamed.UpdatedAt)
			require.Equal(t, int64(100), renamed.PromptTokens)
			require.Equal(t, 0.25, renamed.Cost)

			listed, err := sessions.List(t.Context())
			require.NoError(t, err)
			require.Len(t, listed, 2)
			require.Equal(t, "latest", listed[0].ID)
			require.Equal(t, "older", listed[1].ID)

			last, err := sessions.GetLast(t.Context())
			require.NoError(t, err)
			require.Equal(t, "latest", last.ID)
		})
	}
}

func TestSessionActivityAdvancesTimestamp(t *testing.T) {
	for _, name := range []string{"message", "channel", "save"} {
		t.Run(name, func(t *testing.T) {
			dataDir := t.TempDir()
			t.Cleanup(func() { require.NoError(t, db.Release(dataDir)) })

			conn, err := db.Connect(t.Context(), dataDir)
			require.NoError(t, err)
			_, err = conn.ExecContext(t.Context(), `
				INSERT INTO sessions (id, title, created_at, updated_at)
				VALUES ('active', 'Conversation', 1, 100)
			`)
			require.NoError(t, err)
			queries := db.New(conn)
			sessions := NewService(queries, conn)

			switch name {
			case "message":
				_, err = message.NewService(queries).Create(t.Context(), "active", message.CreateMessageParams{
					Role: message.User,
				})
			case "channel":
				_, err = sessions.SetChannel(t.Context(), "active", "signal")
			case "save":
				var active Session
				active, err = sessions.Get(t.Context(), "active")
				require.NoError(t, err)
				active.Todos = []Todo{{Content: "Next task", Status: TodoStatusPending}}
				_, err = sessions.Save(t.Context(), active)
			}
			require.NoError(t, err)

			updated, err := sessions.Get(t.Context(), "active")
			require.NoError(t, err)
			require.Greater(t, updated.UpdatedAt, int64(100))
		})
	}
}

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

func TestSessionChannelPersists(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	sessions := NewService(db.New(conn), conn)

	created, err := sessions.Create(t.Context(), "channel")
	require.NoError(t, err)
	updated, err := sessions.SetChannel(t.Context(), created.ID, "signal")
	require.NoError(t, err)
	require.Equal(t, "signal", updated.Channel)

	fetched, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "signal", fetched.Channel)
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

func TestSkillDisabledRoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)

	sessions := NewService(db.New(conn), conn)

	disabled, err := sessions.SkillsDisabled(t.Context())
	require.NoError(t, err)
	require.Empty(t, disabled, "a new repository must default to the config")

	require.NoError(t, sessions.SetSkillDisabled(t.Context(), "charmtone", true, false))
	require.NoError(t, sessions.SetSkillDisabled(t.Context(), "pair", true, false))
	require.NoError(t, sessions.SetSkillDisabled(t.Context(), "charmtone", true, false), "disabling twice must be idempotent")

	disabled, err = sessions.SkillsDisabled(t.Context())
	require.NoError(t, err)
	require.Equal(t, []string{"charmtone", "pair"}, disabled)

	// Re-enabling a skill the config does not disable records no enabled
	// override, so a later config disable still applies here.
	require.NoError(t, sessions.SetSkillDisabled(t.Context(), "pair", false, false))
	enabled, err := sessions.SkillsEnabled(t.Context())
	require.NoError(t, err)
	require.Empty(t, enabled)

	// Re-enabling a config-disabled skill records an enabled override so
	// it stays enabled across restarts; disabling removes it again.
	require.NoError(t, sessions.SetSkillDisabled(t.Context(), "charmtone", false, true))
	enabled, err = sessions.SkillsEnabled(t.Context())
	require.NoError(t, err)
	require.Equal(t, []string{"charmtone"}, enabled)

	require.NoError(t, sessions.SetSkillDisabled(t.Context(), "charmtone", true, true))
	enabled, err = sessions.SkillsEnabled(t.Context())
	require.NoError(t, err)
	require.Empty(t, enabled)
}
