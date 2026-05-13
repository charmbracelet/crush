package db

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnect_SharesConnectionForSameDataDir(t *testing.T) {
	t.Cleanup(ResetPool)

	dataDir := t.TempDir()

	conn1, err := Connect(context.Background(), dataDir)
	require.NoError(t, err)

	conn2, err := Connect(context.Background(), dataDir)
	require.NoError(t, err)

	require.Same(t, conn1, conn2, "should return the same *sql.DB for the same data dir")

	// Releasing once should not close the connection.
	require.NoError(t, Release(dataDir))
	require.NoError(t, conn1.PingContext(context.Background()), "connection should still be usable after partial release")

	// Releasing again should close it.
	require.NoError(t, Release(dataDir))
	require.Error(t, conn1.PingContext(context.Background()), "connection should be closed after final release")
}

func TestConnect_SeparateConnectionsForDifferentDataDirs(t *testing.T) {
	t.Cleanup(ResetPool)

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	conn1, err := Connect(context.Background(), dir1)
	require.NoError(t, err)

	conn2, err := Connect(context.Background(), dir2)
	require.NoError(t, err)

	require.NotSame(t, conn1, conn2, "different data dirs should get different connections")

	require.NoError(t, Release(dir1))
	require.NoError(t, Release(dir2))
}

func TestRelease_NoopForUnknownDataDir(t *testing.T) {
	t.Cleanup(ResetPool)

	require.NoError(t, Release("/nonexistent/path"), "releasing unknown data dir should not error")
}

func TestConnect_ExclusiveLockPreventsSecondInstance(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("flock-based lockfile not supported on Windows")
	}

	t.Cleanup(ResetPool)

	dataDir := t.TempDir()

	// First connection succeeds.
	_, err := Connect(t.Context(), dataDir, ConnectOptions{
		ExclusiveLock: true,
	})
	require.NoError(t, err)

	// Simulate a second process by trying to acquire the lockfile
	// directly (same pool would just bump refcount).
	lockPath := filepath.Join(dataDir, "crush.lock")
	_, err = tryExclusiveLock(lockPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "another Crush instance")

	require.NoError(t, Release(dataDir))

	// After release, the lock should be available again.
	lf, err := tryExclusiveLock(lockPath)
	require.NoError(t, err)
	if lf != nil {
		lf.Close()
	}
}

func TestConnect_ExclusiveLock(t *testing.T) {
	t.Cleanup(ResetPool)

	dataDir := t.TempDir()

	conn, err := Connect(context.Background(), dataDir, ConnectOptions{
		ExclusiveLock: true,
	})
	require.NoError(t, err)

	var mode string
	require.NoError(t, conn.QueryRowContext(t.Context(), "PRAGMA locking_mode").Scan(&mode))
	require.Equal(t, "exclusive", mode)

	require.NoError(t, Release(dataDir))
}

func TestConnect_DefaultLockingMode(t *testing.T) {
	t.Cleanup(ResetPool)

	dataDir := t.TempDir()

	conn, err := Connect(context.Background(), dataDir)
	require.NoError(t, err)

	var mode string
	require.NoError(t, conn.QueryRowContext(t.Context(), "PRAGMA locking_mode").Scan(&mode))
	require.Equal(t, "normal", mode)

	require.NoError(t, Release(dataDir))
}
