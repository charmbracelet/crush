package filetracker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

type testEnv struct {
	q        *db.Queries
	sessions session.Service
	svc      Service
}

func setupTest(t *testing.T) *testEnv {
	t.Helper()

	workingDir := t.TempDir()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)

	return &testEnv{
		q:        q,
		sessions: sessions,
		svc:      NewService(q, sessions, workingDir),
	}
}

func (e *testEnv) createSession(t *testing.T, sessionID string) {
	t.Helper()
	_, err := e.q.CreateSession(t.Context(), db.CreateSessionParams{
		ID:    sessionID,
		Title: "Test Session",
	})
	require.NoError(t, err)
}

func createFile(t *testing.T, dir, relPath, content string) string {
	t.Helper()
	absPath := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o755))
	require.NoError(t, os.WriteFile(absPath, []byte(content), 0o644))
	return absPath
}

func TestChangedSinceRead(t *testing.T) {
	env := setupTest(t)
	sessionID := "test-session"
	env.createSession(t, sessionID)

	workingDir := t.TempDir()
	tracker := NewService(env.q, env.sessions, workingDir)
	filePath := createFile(t, workingDir, "a.txt", "hello")

	snapshot, err := SnapshotFromPath(filePath)
	require.NoError(t, err)
	require.NoError(t, tracker.RecordRead(t.Context(), sessionID, filePath, snapshot))

	changed, err := tracker.ChangedSinceRead(t.Context(), sessionID, filePath)
	require.NoError(t, err)
	require.False(t, changed)

	require.NoError(t, os.WriteFile(filePath, []byte("changed"), 0o644))
	changed, err = tracker.ChangedSinceRead(t.Context(), sessionID, filePath)
	require.NoError(t, err)
	require.True(t, changed)
}

func TestInCurrentContext(t *testing.T) {
	env := setupTest(t)
	sessionID := "test-session-context"
	env.createSession(t, sessionID)

	workingDir := t.TempDir()
	tracker := NewService(env.q, env.sessions, workingDir)
	filePath := createFile(t, workingDir, "a.txt", "hello")

	inCtx, err := tracker.InCurrentContext(t.Context(), sessionID, filePath)
	require.NoError(t, err)
	require.False(t, inCtx)

	snapshot, err := SnapshotFromPath(filePath)
	require.NoError(t, err)
	require.NoError(t, tracker.RecordIncludedInContext(t.Context(), sessionID, filePath, snapshot))

	inCtx, err = tracker.InCurrentContext(t.Context(), sessionID, filePath)
	require.NoError(t, err)
	require.True(t, inCtx)

	sess, err := env.sessions.Get(t.Context(), sessionID)
	require.NoError(t, err)
	sess.ContextEpoch++
	_, err = env.sessions.Save(t.Context(), sess)
	require.NoError(t, err)

	inCtx, err = tracker.InCurrentContext(t.Context(), sessionID, filePath)
	require.NoError(t, err)
	require.False(t, inCtx)
}

func TestCanonicalPathSameForRelativeAndAbsolute(t *testing.T) {
	env := setupTest(t)
	sessionID := "test-session-canonical"
	env.createSession(t, sessionID)

	workingDir := t.TempDir()
	tracker := NewService(env.q, env.sessions, workingDir)
	filePath := createFile(t, workingDir, filepath.Join("dir", "a.txt"), "hello")

	absSnapshot, err := SnapshotFromPath(filePath)
	require.NoError(t, err)
	require.NoError(t, tracker.RecordRead(t.Context(), sessionID, filePath, absSnapshot))

	relSnapshot, err := SnapshotFromPath(filePath)
	require.NoError(t, err)
	require.NoError(t, tracker.RecordRead(t.Context(), sessionID, filepath.Join("dir", "a.txt"), relSnapshot))

	files, err := tracker.ListReadFiles(t.Context(), sessionID)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, filepath.Clean(filePath), filepath.Clean(files[0]))
}
