package filetracker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

type testEnv struct {
	ctx        context.Context
	q          *db.Queries
	svc        Service
	workingDir string
}

func setupTest(t *testing.T) *testEnv {
	t.Helper()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	workingDir := t.TempDir()
	return &testEnv{
		ctx:        t.Context(),
		q:          q,
		svc:        NewService(q, workingDir),
		workingDir: workingDir,
	}
}

func (e *testEnv) createSession(t *testing.T, sessionID string) {
	t.Helper()
	_, err := e.q.CreateSession(e.ctx, db.CreateSessionParams{
		ID:    sessionID,
		Title: "Test Session",
	})
	require.NoError(t, err)
}

func TestService_RecordRead(t *testing.T) {
	env := setupTest(t)

	sessionID := "test-session-1"
	path := "/path/to/file.go"
	env.createSession(t, sessionID)

	env.svc.RecordRead(env.ctx, sessionID, path)

	lastRead := env.svc.LastReadTime(env.ctx, sessionID, path)
	require.False(t, lastRead.IsZero(), "expected non-zero time after recording read")
	require.WithinDuration(t, time.Now(), lastRead, 2*time.Second)
}

func TestService_LastReadTime_NotFound(t *testing.T) {
	env := setupTest(t)

	lastRead := env.svc.LastReadTime(env.ctx, "nonexistent-session", "/nonexistent/path")
	require.True(t, lastRead.IsZero(), "expected zero time for unread file")
}

func TestService_RecordRead_UpdatesTimestamp(t *testing.T) {
	env := setupTest(t)

	sessionID := "test-session-2"
	path := "/path/to/file.go"
	env.createSession(t, sessionID)

	env.svc.RecordRead(env.ctx, sessionID, path)
	firstRead := env.svc.LastReadTime(env.ctx, sessionID, path)
	require.False(t, firstRead.IsZero())

	synctest.Test(t, func(t *testing.T) {
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()
		env.svc.RecordRead(env.ctx, sessionID, path)
		secondRead := env.svc.LastReadTime(env.ctx, sessionID, path)

		require.False(t, secondRead.Before(firstRead), "second read time should not be before first")
	})
}

func TestService_RecordRead_DifferentSessions(t *testing.T) {
	env := setupTest(t)

	path := "/shared/file.go"
	session1, session2 := "session-1", "session-2"
	env.createSession(t, session1)
	env.createSession(t, session2)

	env.svc.RecordRead(env.ctx, session1, path)

	lastRead1 := env.svc.LastReadTime(env.ctx, session1, path)
	require.False(t, lastRead1.IsZero())

	lastRead2 := env.svc.LastReadTime(env.ctx, session2, path)
	require.True(t, lastRead2.IsZero(), "session 2 should not see session 1's read")
}

func TestService_RecordRead_DifferentPaths(t *testing.T) {
	env := setupTest(t)

	sessionID := "test-session-3"
	path1, path2 := "/path/to/file1.go", "/path/to/file2.go"
	env.createSession(t, sessionID)

	env.svc.RecordRead(env.ctx, sessionID, path1)

	lastRead1 := env.svc.LastReadTime(env.ctx, sessionID, path1)
	require.False(t, lastRead1.IsZero())

	lastRead2 := env.svc.LastReadTime(env.ctx, sessionID, path2)
	require.True(t, lastRead2.IsZero(), "path2 should not be recorded")
}

// TestService_PathsRelativeToWorkingDir checks that read records do not
// depend on the process working directory. A server can be started from any
// directory and a respawned one from yet another, so records written by a
// service running in one directory must be found by one running in another.
func TestService_PathsRelativeToWorkingDir(t *testing.T) {
	env := setupTest(t)

	sessionID := "test-session-4"
	path := filepath.Join(env.workingDir, "src", "main.go")
	env.createSession(t, sessionID)

	t.Chdir(t.TempDir())
	NewService(env.q, env.workingDir).RecordRead(env.ctx, sessionID, path)

	deeper := filepath.Join(t.TempDir(), "deeper")
	require.NoError(t, os.Mkdir(deeper, 0o755))
	t.Chdir(deeper)
	svc := NewService(env.q, env.workingDir)

	lastRead := svc.LastReadTime(env.ctx, sessionID, path)
	require.False(t, lastRead.IsZero(), "read recorded from another directory should be found")

	files, err := svc.ListReadFiles(env.ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, []string{path}, files)

	// Rows are stored relative to the working directory, matching the rows
	// local mode, which runs in the workspace, has always written.
	rows, err := env.q.ListSessionReadFiles(env.ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, filepath.Join("src", "main.go"), rows[0].Path)
}

func TestService_RelativeWorkingDir(t *testing.T) {
	env := setupTest(t)

	sessionID := "test-session-5"
	path := filepath.Join(env.workingDir, "main.go")
	env.createSession(t, sessionID)

	t.Chdir(filepath.Dir(env.workingDir))
	svc := NewService(env.q, filepath.Base(env.workingDir))
	svc.RecordRead(env.ctx, sessionID, path)

	files, err := svc.ListReadFiles(env.ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, []string{path}, files)
}
