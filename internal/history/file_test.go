package history

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (Service, *db.Queries) {
	t.Helper()
	dataDir := t.TempDir()
	t.Cleanup(func() {
		// Release only this test's pooled connection; a global
		// db.ResetPool() here would close the databases of tests still
		// running in parallel.
		require.NoError(t, db.Release(dataDir))
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	q := db.New(conn)
	return NewService(q, conn), q
}

func createSession(t *testing.T, q *db.Queries, id string) {
	t.Helper()
	_, err := q.CreateSession(t.Context(), db.CreateSessionParams{
		ID:    id,
		Title: id,
	})
	require.NoError(t, err)
}

func TestCreateVersion_StartsAtInitialVersion(t *testing.T) {
	t.Parallel()
	svc, q := newTestService(t)
	createSession(t, q, "s1")

	f, err := svc.CreateVersion(t.Context(), "s1", "/a.go", "one")
	require.NoError(t, err)
	require.Equal(t, int64(InitialVersion), f.Version)
	require.Equal(t, "one", f.Content)
}

func TestCreateVersion_IncrementsAcrossSessions(t *testing.T) {
	t.Parallel()
	svc, q := newTestService(t)
	createSession(t, q, "s1")
	createSession(t, q, "s2")

	_, err := svc.Create(t.Context(), "s1", "/a.go", "v0")
	require.NoError(t, err)
	f1, err := svc.CreateVersion(t.Context(), "s1", "/a.go", "v1")
	require.NoError(t, err)
	require.Equal(t, int64(1), f1.Version)

	// The version counter is shared by every session that touches a path.
	f2, err := svc.CreateVersion(t.Context(), "s2", "/a.go", "v2")
	require.NoError(t, err)
	require.Equal(t, int64(2), f2.Version)
	require.Equal(t, "v2", f2.Content)

	// Other paths keep their own counters.
	other, err := svc.CreateVersion(t.Context(), "s2", "/b.go", "b0")
	require.NoError(t, err)
	require.Equal(t, int64(InitialVersion), other.Version)

	latest, err := q.GetLatestFileVersionByPath(t.Context(), "/a.go")
	require.NoError(t, err)
	require.Equal(t, int64(2), latest)

	missing, err := q.GetLatestFileVersionByPath(t.Context(), "/missing.go")
	require.NoError(t, err)
	require.Equal(t, int64(-1), missing)
}
