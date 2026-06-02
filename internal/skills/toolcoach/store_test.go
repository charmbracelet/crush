package toolcoach

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

	_, err = conn.Exec(`
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
		CREATE TABLE toolcoach_effectiveness (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pattern_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			fired_count INTEGER NOT NULL DEFAULT 0,
			acted_count INTEGER NOT NULL DEFAULT 0,
			ignored_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
		);
		CREATE INDEX idx_toolcoach_eff_pattern ON toolcoach_effectiveness (pattern_id);
		CREATE INDEX idx_toolcoach_eff_session ON toolcoach_effectiveness (session_id);
	`)
	require.NoError(t, err)
	return conn
}

func TestStore_RecordAndGet(t *testing.T) {
	t.Parallel()
	conn := setupTestDB(t)
	q := db.New(conn)
	store := NewStore(q)
	ctx := context.Background()

	// Seed a session so the FK constraint is satisfied.
	_, err := conn.Exec("INSERT INTO sessions (id) VALUES ('sid1')")
	require.NoError(t, err)

	err = store.RecordSessionEffectiveness(ctx, "sid1", "edit_without_view", 10, 7, 3)
	require.NoError(t, err)

	rec, err := store.GetPatternEffectiveness(ctx, "edit_without_view", 24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, "edit_without_view", rec.PatternID)
	require.Equal(t, int64(10), rec.TotalFired)
	require.Equal(t, int64(7), rec.TotalActed)
	require.Equal(t, int64(3), rec.TotalIgnored)
}

func TestStore_GetPatternEffectiveness_NoData(t *testing.T) {
	t.Parallel()
	conn := setupTestDB(t)
	q := db.New(conn)
	store := NewStore(q)
	ctx := context.Background()

	rec, err := store.GetPatternEffectiveness(ctx, "missing_pattern", 24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, "missing_pattern", rec.PatternID)
	require.Equal(t, int64(0), rec.TotalFired)
	require.Equal(t, int64(0), rec.TotalActed)
	require.Equal(t, int64(0), rec.TotalIgnored)
}

func TestStore_GetPatternEffectiveness_LookbackFilters(t *testing.T) {
	t.Parallel()
	conn := setupTestDB(t)
	q := db.New(conn)
	store := NewStore(q)
	ctx := context.Background()

	_, err := conn.Exec("INSERT INTO sessions (id) VALUES ('sid1')")
	require.NoError(t, err)

	// Insert old record.
	_, err = conn.Exec(
		"INSERT INTO toolcoach_effectiveness (pattern_id, session_id, fired_count, acted_count, ignored_count, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"edit_without_view", "sid1", 5, 1, 4, time.Now().Add(-48*time.Hour).Unix(),
	)
	require.NoError(t, err)

	// Insert recent record.
	_, err = conn.Exec(
		"INSERT INTO toolcoach_effectiveness (pattern_id, session_id, fired_count, acted_count, ignored_count, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"edit_without_view", "sid1", 10, 8, 2, time.Now().Add(-1*time.Hour).Unix(),
	)
	require.NoError(t, err)

	// Lookback of 24 hours should only include the recent record.
	rec, err := store.GetPatternEffectiveness(ctx, "edit_without_view", 24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(10), rec.TotalFired)
	require.Equal(t, int64(8), rec.TotalActed)
	require.Equal(t, int64(2), rec.TotalIgnored)

	// Lookback of 72 hours should include both.
	rec, err = store.GetPatternEffectiveness(ctx, "edit_without_view", 72*time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(15), rec.TotalFired)
	require.Equal(t, int64(9), rec.TotalActed)
	require.Equal(t, int64(6), rec.TotalIgnored)
}

func TestStore_Prune(t *testing.T) {
	t.Parallel()
	conn := setupTestDB(t)
	q := db.New(conn)
	store := NewStore(q)
	store.SetDB(conn)
	ctx := context.Background()

	_, err := conn.Exec("INSERT INTO sessions (id) VALUES ('sid1')")
	require.NoError(t, err)

	_, err = conn.Exec(
		"INSERT INTO toolcoach_effectiveness (pattern_id, session_id, fired_count, created_at) VALUES (?, ?, ?, ?)",
		"edit_without_view", "sid1", 1, time.Now().Add(-48*time.Hour).Unix(),
	)
	require.NoError(t, err)

	n, err := store.Prune(ctx, time.Now().Add(-24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}

func TestStore_Prune_NoDB(t *testing.T) {
	t.Parallel()
	conn := setupTestDB(t)
	q := db.New(conn)
	store := NewStore(q)
	ctx := context.Background()

	_, err := store.Prune(ctx, time.Now())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no database connection")
}

func TestStore_NoQuerier(t *testing.T) {
	t.Parallel()
	store := NewStore(nil)
	ctx := context.Background()

	err := store.RecordSessionEffectiveness(ctx, "sid", "pat", 1, 0, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no querier configured")

	_, err = store.GetPatternEffectiveness(ctx, "pat", 24*time.Hour)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no querier configured")
}
