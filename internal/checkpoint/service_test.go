package checkpoint

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/taigrr/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateSnapshot(t *testing.T) {
	t.Parallel()

	// Create temp dir for project
	projectDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(projectDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0o644))

	// Create in-memory sqlite db with schema
	conn, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(0)")
	require.NoError(t, err)
	defer conn.Close()

	// Run migrations
	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY
		);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			message_id TEXT NOT NULL,
			parent_snapshot_id TEXT,
			git_commit_hash TEXT NOT NULL,
			description TEXT,
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_snapshots_session_id ON snapshots(session_id);
		CREATE INDEX IF NOT EXISTS idx_snapshots_message_id ON snapshots(message_id);
	`)
	require.NoError(t, err)

	// Insert test session and message
	_, err = conn.Exec("INSERT INTO sessions (id) VALUES ('test-session')")
	require.NoError(t, err)
	_, err = conn.Exec("INSERT INTO messages (id, session_id) VALUES ('test-msg', 'test-session')")
	require.NoError(t, err)

	q := db.New(conn)

	// Create service
	svc, err := NewService(ServiceConfig{
		Enabled:    true,
		ProjectDir: projectDir,
	}, q, conn)
	require.NoError(t, err)
	require.True(t, svc.IsEnabled())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create snapshot
	snap, err := svc.CreateSnapshot(ctx, "test-session", "test-msg", "Test snapshot")
	require.NoError(t, err, "CreateSnapshot should succeed")
	require.NotNil(t, snap)
	require.NotEmpty(t, snap.ID)
	require.NotEmpty(t, snap.GitCommitHash)
	t.Logf("Created snapshot: ID=%s, GitCommit=%s", snap.ID, snap.GitCommitHash)

	// Verify it was persisted
	snaps, err := svc.ListSnapshots(ctx, "test-session")
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	require.Equal(t, snap.ID, snaps[0].ID)
}
