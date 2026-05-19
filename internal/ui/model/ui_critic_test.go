package model

import (
	"context"
	"database/sql"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/skills/critic"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
	"modernc.org/sqlite"
)

func init() {
	sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
		_, err := conn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON", nil)
		return err
	})
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.ExecContext(context.Background(), `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			parent_session_id TEXT,
			title TEXT NOT NULL DEFAULT '',
			message_count INTEGER NOT NULL DEFAULT 0,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			cost REAL NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT 0,
			summary_message_id TEXT,
			todos TEXT
		);
		CREATE TABLE messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			parts TEXT NOT NULL,
			model TEXT,
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0,
			finished_at INTEGER,
			provider TEXT,
			is_summary_message INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE critic_reviews (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			message_id TEXT NOT NULL,
			verdict TEXT NOT NULL,
			confidence REAL NOT NULL DEFAULT 0.0,
			concerns TEXT NOT NULL DEFAULT '[]',
			summary TEXT NOT NULL DEFAULT '',
			diff_snapshot TEXT NOT NULL DEFAULT '',
			lsp_diagnostics TEXT NOT NULL DEFAULT '[]',
			created_at INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX idx_critic_reviews_session_id ON critic_reviews (session_id);
		CREATE INDEX idx_critic_reviews_message_id ON critic_reviews (message_id);
	`)
	require.NoError(t, err)
	return conn
}

func newTestUIWithCriticStore(t *testing.T, store *critic.Store) *UI {
	t.Helper()
	sty := styles.DefaultStyles()
	com := &common.Common{
		App: &app.App{
			CriticStore: store,
		},
		Styles: &sty,
	}
	return &UI{com: com}
}

func TestCriticVerdictForMessage(t *testing.T) {
	t.Parallel()

	conn := setupTestDB(t)
	q := db.New(conn)
	store := critic.NewStore(q)
	ctx := context.Background()

	_, err := conn.ExecContext(ctx, "INSERT INTO sessions (id, title) VALUES ('s1', 'test')")
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "INSERT INTO messages (id, session_id, role, parts) VALUES ('m1', 's1', 'assistant', '[]')")
	require.NoError(t, err)

	// Create a review for message m1.
	fb := &critic.CriticFeedback{
		Verdict:    "approve",
		Confidence: 0.95,
		Summary:    "Looks good",
	}
	_, err = store.Create(ctx, "s1", "m1", fb, "", nil)
	require.NoError(t, err)

	ui := newTestUIWithCriticStore(t, store)

	t.Run("existing review", func(t *testing.T) {
		verdict := ui.criticVerdictForMessage("m1")
		require.Equal(t, "approve", verdict)
	})

	t.Run("no review", func(t *testing.T) {
		verdict := ui.criticVerdictForMessage("nonexistent")
		require.Equal(t, "", verdict)
	})
}

func TestCriticVerdictForMessage_NoStore(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles()
	com := &common.Common{
		App:    &app.App{}, // no CriticStore
		Styles: &sty,
	}
	ui := &UI{com: com}

	verdict := ui.criticVerdictForMessage("m1")
	require.Equal(t, "", verdict)
}

func TestCriticVerdictForMessage_MultipleVerdicts(t *testing.T) {
	t.Parallel()

	conn := setupTestDB(t)
	q := db.New(conn)
	store := critic.NewStore(q)
	ctx := context.Background()

	_, err := conn.ExecContext(ctx, "INSERT INTO sessions (id, title) VALUES ('s1', 'test')")
	require.NoError(t, err)
	for _, m := range []string{"m1", "m2", "m3"} {
		_, err = conn.ExecContext(ctx, "INSERT INTO messages (id, session_id, role, parts) VALUES (?, 's1', 'assistant', '[]')", m)
		require.NoError(t, err)
	}

	// Create reviews with different verdicts.
	_, err = store.Create(ctx, "s1", "m1", &critic.CriticFeedback{Verdict: "approve", Confidence: 0.9}, "", nil)
	require.NoError(t, err)
	_, err = store.Create(ctx, "s1", "m2", &critic.CriticFeedback{Verdict: "revise", Confidence: 0.6}, "", nil)
	require.NoError(t, err)
	_, err = store.Create(ctx, "s1", "m3", &critic.CriticFeedback{Verdict: "halt", Confidence: 0.3}, "", nil)
	require.NoError(t, err)

	ui := newTestUIWithCriticStore(t, store)

	require.Equal(t, "approve", ui.criticVerdictForMessage("m1"))
	require.Equal(t, "revise", ui.criticVerdictForMessage("m2"))
	require.Equal(t, "halt", ui.criticVerdictForMessage("m3"))
	require.Equal(t, "", ui.criticVerdictForMessage("m4"))
}
