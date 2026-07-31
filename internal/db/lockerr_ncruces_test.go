//go:build !((darwin && (amd64 || arm64)) || (freebsd && (amd64 || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (windows && (386 || amd64 || arm64)))

package db

import (
	"fmt"
	"testing"

	"github.com/ncruces/go-sqlite3"
	"github.com/stretchr/testify/require"
)

// TestIsLockProtocolError_Positive pins the errors that must trigger the
// rollback-journal fallback. This driver reports failures either as a
// bare code value or as a *sqlite3.Error, and narrows Code() to the
// primary code, so both shapes and the extended IOERR_LOCK code have to
// be matched.
func TestIsLockProtocolError_Positive(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		sqlite3.PROTOCOL,
		sqlite3.IOERR_LOCK,
		fmt.Errorf("failed to set pragma: %w", error(sqlite3.PROTOCOL)),
		fmt.Errorf("failed to set pragma: %w", error(sqlite3.IOERR_LOCK)),
	} {
		require.True(t, isLockProtocolError(err), "%v must be treated as a lock failure", err)
	}

	// A locking failure must also carry the network-filesystem hint.
	hinted := maybeLockHint(sqlite3.IOERR_LOCK)
	require.ErrorIs(t, hinted, sqlite3.IOERR_LOCK)
	require.Contains(t, hinted.Error(), "data_directory")
}

// TestIsLockProtocolError_UnrelatedCodes guards against matching so
// broadly that ordinary I/O errors force an unnecessary fallback.
func TestIsLockProtocolError_UnrelatedCodes(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		sqlite3.BUSY,
		sqlite3.IOERR,
		sqlite3.IOERR_READ,
		sqlite3.CANTOPEN,
	} {
		require.False(t, isLockProtocolError(err), "%v must not be treated as a lock failure", err)
	}
}
