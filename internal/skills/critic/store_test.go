package critic

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/db"
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

	// Run migrations to create tables.
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

func TestStore_CreateAndGet(t *testing.T) {
	t.Parallel()
	conn := setupTestDB(t)
	q := db.New(conn)
	store := NewStore(q)
	ctx := context.Background()

	// Seed a session and message so FKs are satisfied.
	_, err := conn.ExecContext(ctx, "INSERT INTO sessions (id, title) VALUES ('s1', 'test')")
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "INSERT INTO messages (id, session_id, role, parts) VALUES ('m1', 's1', 'assistant', '[]')")
	require.NoError(t, err)

	feedback := &CriticFeedback{
		Verdict:    "approve",
		Confidence: 0.95,
		Concerns: []CriticConcern{
			{Severity: "major", Dimension: "style", Summary: "naming", Suggestion: "use camelCase"},
		},
		Summary: "Looks good",
	}
	diags := []DiagnosticSnapshot{{Path: "main.go", Severity: "error", Message: "unused var", Line: 5}}

	record, err := store.Create(ctx, "s1", "m1", feedback, "diff", diags)
	require.NoError(t, err)
	require.Equal(t, "approve", record.Verdict)
	require.InDelta(t, 0.95, record.Confidence, 0.001)
	require.Equal(t, "Looks good", record.Summary)
	require.Len(t, record.Concerns, 1)
	require.Len(t, record.LSPDiagnostics, 1)
	require.Equal(t, "diff", record.DiffSnapshot)

	// Retrieve by message ID.
	got, err := store.GetByMessageID(ctx, "m1")
	require.NoError(t, err)
	require.Equal(t, record.ID, got.ID)
	require.Equal(t, "approve", got.Verdict)
}

func TestStore_ListBySession(t *testing.T) {
	t.Parallel()
	conn := setupTestDB(t)
	q := db.New(conn)
	store := NewStore(q)
	ctx := context.Background()

	_, err := conn.ExecContext(ctx, "INSERT INTO sessions (id, title) VALUES ('s1', 'test')")
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "INSERT INTO messages (id, session_id, role, parts) VALUES ('m1', 's1', 'assistant', '[]')")
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "INSERT INTO messages (id, session_id, role, parts) VALUES ('m2', 's1', 'assistant', '[]')")
	require.NoError(t, err)

	fb1 := &CriticFeedback{Verdict: "approve", Confidence: 0.9}
	fb2 := &CriticFeedback{Verdict: "revise", Confidence: 0.5}

	_, err = store.Create(ctx, "s1", "m1", fb1, "", nil)
	require.NoError(t, err)
	_, err = store.Create(ctx, "s1", "m2", fb2, "", nil)
	require.NoError(t, err)

	records, err := store.ListBySession(ctx, "s1")
	require.NoError(t, err)
	require.Len(t, records, 2)
	// Both verdicts should be present; order depends on created_at.
	verdicts := map[string]bool{}
	for _, r := range records {
		verdicts[r.Verdict] = true
	}
	require.True(t, verdicts["approve"])
	require.True(t, verdicts["revise"])
}

func TestStore_GetByMessageID_NotFound(t *testing.T) {
	t.Parallel()
	conn := setupTestDB(t)
	q := db.New(conn)
	store := NewStore(q)
	ctx := context.Background()

	_, err := store.GetByMessageID(ctx, "nonexistent")
	require.Error(t, err)
}

func TestStore_Prune(t *testing.T) {
	t.Parallel()
	conn := setupTestDB(t)
	q := db.New(conn)
	store := NewStore(q)
	store.SetDB(conn)
	ctx := context.Background()

	_, err := conn.ExecContext(ctx, "INSERT INTO sessions (id, title) VALUES ('s1', 'test')")
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "INSERT INTO messages (id, session_id, role, parts) VALUES ('m1', 's1', 'assistant', '[]')")
	require.NoError(t, err)

	// Insert an old review (created 10 days ago).
	_, err = conn.ExecContext(ctx,
		"INSERT INTO critic_reviews (id, session_id, message_id, verdict, confidence, created_at) VALUES ('r1', 's1', 'm1', 'approve', 0.9, ?)",
		time.Now().AddDate(0, 0, -10).Unix(),
	)
	require.NoError(t, err)

	// Insert a recent review (created today).
	_, err = conn.ExecContext(ctx,
		"INSERT INTO critic_reviews (id, session_id, message_id, verdict, confidence, created_at) VALUES ('r2', 's1', 'm1', 'approve', 0.9, ?)",
		time.Now().Unix(),
	)
	require.NoError(t, err)

	// Prune with 5-day cutoff — should delete only the old review.
	n, err := store.Prune(ctx, time.Now().AddDate(0, 0, -5))
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	// Verify only the recent review remains.
	rows, err := conn.QueryContext(ctx, "SELECT id FROM critic_reviews")
	require.NoError(t, err)
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.Len(t, ids, 1)
	require.Equal(t, "r2", ids[0])
}

func TestStore_Prune_NoDB(t *testing.T) {
	t.Parallel()
	q := db.New(setupTestDB(t))
	store := NewStore(q) // no SetDB called
	ctx := context.Background()

	_, err := store.Prune(ctx, time.Now())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no database connection configured")
}
