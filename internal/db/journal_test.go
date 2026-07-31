package db

import (
	"context"
	"database/sql"
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

// TestOpenAndPing_RollbackJournal covers the fallback Connect uses when
// WAL is unavailable: opening straight into the rollback journal, and
// converting a database that already exists in WAL mode.
func TestOpenAndPing_RollbackJournal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	journalMode := func(t *testing.T, conn *sql.DB) string {
		t.Helper()
		var mode string
		require.NoError(t, conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode))
		return mode
	}

	fresh, err := openAndPing(ctx, filepath.Join(t.TempDir(), "fresh.db"), "DELETE")
	require.NoError(t, err)
	defer fresh.Close()
	require.Equal(t, "delete", journalMode(t, fresh))

	// A database created in WAL must convert cleanly, which is what
	// happens when a data directory moves onto a network filesystem.
	path := filepath.Join(t.TempDir(), "converted.db")
	wal, err := openAndPing(ctx, path, "")
	require.NoError(t, err)
	require.Equal(t, "wal", journalMode(t, wal))
	require.NoError(t, wal.Close())

	converted, err := openAndPing(ctx, path, "DELETE")
	require.NoError(t, err)
	defer converted.Close()
	require.Equal(t, "delete", journalMode(t, converted))
}

// TestVerifyJournalMode_StuckInWAL covers the case where the rollback
// journal is requested but the database stays in WAL. Converting away
// from WAL has to read the existing write-ahead log, which needs the
// very locking the fallback is trying to avoid, so this must be a
// distinct terminal error rather than a silent success.
func TestVerifyJournalMode_StuckInWAL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	conn, err := openAndPing(ctx, filepath.Join(t.TempDir(), "stuck.db"), "")
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, verifyJournalMode(ctx, conn, ""), "WAL was requested and applied")

	err = verifyJournalMode(ctx, conn, "DELETE")
	require.ErrorIs(t, err, errWALStuck)
	require.False(t, isWALLockFailure(err), "a stuck WAL must not trigger another fallback attempt")
}

// TestMaybeLockHint_StuckInWAL checks that a stuck WAL gets advice
// matching its cause. Relocating the data directory alone does not help
// when the existing database is the thing that cannot be converted.
func TestMaybeLockHint_StuckInWAL(t *testing.T) {
	t.Parallel()

	hinted := maybeLockHint(errWALStuck)
	require.ErrorIs(t, hinted, errWALStuck)
	require.Contains(t, hinted.Error(), "copy the data directory")
	require.Contains(t, hinted.Error(), "data_directory")
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

	// The hint suggests a network filesystem rather than asserting one,
	// since missing locking has other causes.
	require.Contains(t, hinted.Error(), "If the data directory is on one")
}
