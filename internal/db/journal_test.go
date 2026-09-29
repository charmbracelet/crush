package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPragmas_DefaultsToWAL(t *testing.T) {
	t.Parallel()
	require.Equal(t, "WAL", pragmas("")["journal_mode"])
	require.Equal(t, "DELETE", pragmas("DELETE")["journal_mode"])
	// The rest of the pragmas are independent of journal mode.
	require.Equal(t, "ON", pragmas("")["foreign_keys"])
	require.Equal(t, "ON", pragmas("DELETE")["foreign_keys"])
}

func TestConnect_UsesWALByDefault(t *testing.T) {
	t.Cleanup(ResetPool)

	dataDir := t.TempDir()
	conn, err := Connect(context.Background(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() { Release(dataDir) })

	var mode string
	require.NoError(t, conn.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&mode))
	require.Equal(t, "wal", mode)
}

func TestIsLockProtocolError(t *testing.T) {
	t.Parallel()

	// A plain, non-SQLite error is never a lock-protocol error.
	require.False(t, isLockProtocolError(errors.New("boom")))
	require.False(t, isLockProtocolError(nil))

	// A real SQLite error surfaced by the driver must be recognized so
	// the WAL fallback and the filesystem hint engage.
	db, err := openDB(filepath.Join(t.TempDir(), "x.db"), "")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ExecContext(context.Background(), "this is not sql")
	require.Error(t, err)
	require.False(t, isLockProtocolError(err), "a syntax error is not a lock-protocol error")
}

func TestMaybeLockHint(t *testing.T) {
	t.Parallel()

	plain := errors.New("boom")
	require.Same(t, plain, maybeLockHint(plain), "non-lock errors pass through untouched")

	// A WAL-unavailable error gets the filesystem hint appended while
	// remaining matchable via errors.Is.
	hinted := maybeLockHint(errWALUnavailable)
	require.ErrorIs(t, hinted, errWALUnavailable)
	require.Contains(t, hinted.Error(), "data_directory")
}
