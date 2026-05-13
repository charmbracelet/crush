// This is a manual test helper — run it to see the mmap probe and
// exclusive lock auto-detection in action.
//
//	go test ./internal/db/ -run TestMmapProbe -v
package db

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMmapProbe_Available(t *testing.T) {
	dir := t.TempDir()
	result := mmapAvailable(dir)
	fmt.Fprintf(os.Stderr, "mmap available in %s: %v\n", dir, result)
	require.True(t, result, "mmap should work in a normal temp dir")
}

func TestMmapProbe_FallbackToExclusive(t *testing.T) {
	t.Cleanup(ResetPool)

	dataDir := t.TempDir()
	conn, err := Connect(t.Context(), dataDir, ConnectOptions{
		ExclusiveLock: true,
	})
	require.NoError(t, err)

	var mode string
	require.NoError(t, conn.QueryRowContext(t.Context(), "PRAGMA locking_mode").Scan(&mode))
	fmt.Fprintf(os.Stderr, "locking_mode: %s\n", mode)
	require.Equal(t, "exclusive", mode)

	// Verify the DB is fully functional.
	_, err = conn.ExecContext(t.Context(), "CREATE TABLE test_probe (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), "INSERT INTO test_probe (id) VALUES (1)")
	require.NoError(t, err)

	var id int
	require.NoError(t, conn.QueryRowContext(t.Context(), "SELECT id FROM test_probe").Scan(&id))
	require.Equal(t, 1, id)
	fmt.Fprintf(os.Stderr, "DB read/write OK in exclusive mode\n")

	require.NoError(t, Release(dataDir))
}
