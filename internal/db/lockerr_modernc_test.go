//go:build (darwin && (amd64 || arm64)) || (freebsd && (amd64 || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (windows && (386 || amd64 || arm64))

package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	sqlite "modernc.org/sqlite"
)

// TestIsLockResultCode pins which result codes trigger the
// rollback-journal fallback. Matching too narrowly leaves users on
// network filesystems crashing at startup; matching too broadly forces
// an unnecessary fallback on ordinary I/O errors.
func TestIsLockResultCode(t *testing.T) {
	t.Parallel()

	require.True(t, isLockResultCode(15), "SQLITE_PROTOCOL")
	require.True(t, isLockResultCode(3850), "SQLITE_IOERR_LOCK")

	require.False(t, isLockResultCode(5), "SQLITE_BUSY")
	require.False(t, isLockResultCode(10), "SQLITE_IOERR")
	require.False(t, isLockResultCode(14), "SQLITE_CANTOPEN")
	require.False(t, isLockResultCode(266), "SQLITE_IOERR_READ")
}

// TestModerncReportsExtendedCodes guards the assumption that makes
// SQLITE_IOERR_LOCK (3850) reachable: this driver turns on extended
// result codes, so Code() returns the full extended value rather than
// the primary code. If that ever changes, matching on 3850 silently
// stops working and the WAL fallback goes dead.
func TestModerncReportsExtendedCodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := openDB(filepath.Join(t.TempDir(), "codes.db"), "")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, "create table t(a integer primary key)")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "insert into t(a) values (1)")
	require.NoError(t, err)

	// A duplicate primary key reports SQLITE_CONSTRAINT_PRIMARYKEY
	// (1555). The primary code alone would be SQLITE_CONSTRAINT (19).
	_, err = db.ExecContext(ctx, "insert into t(a) values (1)")
	require.Error(t, err)

	sqliteErr, ok := errors.AsType[*sqlite.Error](err)
	require.True(t, ok, "driver must surface *sqlite.Error")
	require.Equal(t, 1555, sqliteErr.Code(),
		"driver must report extended result codes, not the primary code")
}
