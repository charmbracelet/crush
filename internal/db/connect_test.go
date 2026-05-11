package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnect_SharesConnectionForSameDataDir(t *testing.T) {
	t.Cleanup(ResetPool)

	dataDir := t.TempDir()
	ctx := context.Background()

	conn1, err := Connect(ctx, dataDir)
	require.NoError(t, err)

	conn2, err := Connect(ctx, dataDir)
	require.NoError(t, err)

	// Same dataDir should return the same connection.
	require.Same(t, conn1, conn2)

	// Releasing once should not close the connection (still has one ref).
	Release(dataDir)
	require.NoError(t, conn1.Ping())

	// Releasing again should close the connection.
	Release(dataDir)
	require.Error(t, conn1.Ping())
}

func TestConnect_SeparateConnectionsForDifferentDataDirs(t *testing.T) {
	t.Cleanup(ResetPool)

	ctx := context.Background()

	dataDir1 := t.TempDir()
	dataDir2 := t.TempDir()

	conn1, err := Connect(ctx, dataDir1)
	require.NoError(t, err)

	conn2, err := Connect(ctx, dataDir2)
	require.NoError(t, err)

	// Different dataDirs should return different connections.
	require.NotSame(t, conn1, conn2)

	Release(dataDir1)
	Release(dataDir2)
}

func TestRelease_NoopForUnknownDataDir(t *testing.T) {
	t.Cleanup(ResetPool)

	// Should not panic or error for unknown paths.
	Release("/nonexistent/path")
}
