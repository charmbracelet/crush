package history

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (Service, *db.Queries) {
	t.Helper()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	svc := NewService(q, conn)

	return svc, q
}

func createTestSession(t *testing.T, q *db.Queries) string {
	t.Helper()
	id := uuid.New().String()
	_, err := q.CreateSession(t.Context(), db.CreateSessionParams{
		ID:    id,
		Title: "test session",
	})
	require.NoError(t, err)
	return id
}

func TestCreate(t *testing.T) {
	t.Parallel()
	svc, q := setupTest(t)
	sessionID := createTestSession(t, q)

	file, err := svc.Create(t.Context(), sessionID, "/tmp/test.txt", "hello")
	require.NoError(t, err)
	require.Equal(t, int64(0), file.Version)
	require.Equal(t, "hello", file.Content)
	require.Equal(t, sessionID, file.SessionID)
}

func TestCreateVersion(t *testing.T) {
	t.Parallel()
	svc, q := setupTest(t)
	sessionID := createTestSession(t, q)

	_, err := svc.Create(t.Context(), sessionID, "/tmp/test.txt", "v0")
	require.NoError(t, err)

	file, err := svc.CreateVersion(t.Context(), sessionID, "/tmp/test.txt", "v1")
	require.NoError(t, err)
	require.Equal(t, int64(1), file.Version)
	require.Equal(t, "v1", file.Content)
}

// TestCreateDuplicateVersion0 reproduces the bug: calling Create() twice
// for the same (path, session_id) causes a UNIQUE constraint violation on
// version 0, since createNewFile() in edit.go does not guard against it.
func TestCreateDuplicateVersion0(t *testing.T) {
	t.Parallel()
	svc, q := setupTest(t)
	sessionID := createTestSession(t, q)

	_, err := svc.Create(t.Context(), sessionID, "/tmp/test.txt", "first")
	require.NoError(t, err)

	// Second Create() for the same path+session should still succeed
	// (the retry loop bumps version), but currently it may fail after
	// 3 retries if versions 0, 1, 2 are all taken.
	_, err = svc.Create(t.Context(), sessionID, "/tmp/test.txt", "second")
	require.NoError(t, err, "Create() should handle duplicate version 0 gracefully")
}

// TestCreateVersionCrossSessionCollision reproduces the bug where
// CreateVersion() uses ListFilesByPath (which queries ALL sessions) to
// determine the next version number, but the UNIQUE constraint is per-session.
// This can cause version collisions when two sessions edit the same file.
func TestCreateVersionCrossSessionCollision(t *testing.T) {
	t.Parallel()
	svc, q := setupTest(t)
	sessionA := createTestSession(t, q)
	sessionB := createTestSession(t, q)

	// Session A: create file with versions 0, 1, 2.
	_, err := svc.Create(t.Context(), sessionA, "/tmp/shared.txt", "a-v0")
	require.NoError(t, err)
	_, err = svc.CreateVersion(t.Context(), sessionA, "/tmp/shared.txt", "a-v1")
	require.NoError(t, err)
	_, err = svc.CreateVersion(t.Context(), sessionA, "/tmp/shared.txt", "a-v2")
	require.NoError(t, err)

	// Session B: create the same file. Create() inserts version 0 for session B.
	_, err = svc.Create(t.Context(), sessionB, "/tmp/shared.txt", "b-v0")
	require.NoError(t, err)

	// Session B: CreateVersion() calls ListFilesByPath which returns
	// ALL versions across both sessions. The max version is 2 (from session A),
	// so it tries to insert version 3 for session B. This succeeds but
	// leaves a gap (session B has versions 0 and 3).
	file, err := svc.CreateVersion(t.Context(), sessionB, "/tmp/shared.txt", "b-v1")
	require.NoError(t, err)

	// The version should ideally be 1 (session B's second version), not 3.
	// This assertion documents the current buggy behavior where version
	// numbers leak across sessions.
	require.Equal(t, int64(1), file.Version,
		"CreateVersion should use per-session version numbers, not cross-session")
}

// TestCreateDuplicateVersion0ExhaustsRetries shows that when versions 0, 1,
// and 2 already exist for (path, session_id), calling Create() (which starts
// at version 0 and retries 3 times) exhausts all retries and fails.
func TestCreateDuplicateVersion0ExhaustsRetries(t *testing.T) {
	t.Parallel()
	svc, q := setupTest(t)
	sessionID := createTestSession(t, q)

	// Create versions 0, 1, 2.
	_, err := svc.Create(t.Context(), sessionID, "/tmp/test.txt", "v0")
	require.NoError(t, err)
	_, err = svc.CreateVersion(t.Context(), sessionID, "/tmp/test.txt", "v1")
	require.NoError(t, err)
	_, err = svc.CreateVersion(t.Context(), sessionID, "/tmp/test.txt", "v2")
	require.NoError(t, err)

	// Now Create() again — tries version 0, 1, 2, all taken → fails.
	_, err = svc.Create(t.Context(), sessionID, "/tmp/test.txt", "oops")
	require.NoError(t, err, "Create() should succeed even when versions 0-2 exist")
}
