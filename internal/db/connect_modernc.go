//go:build (darwin && (amd64 || arm64)) || (freebsd && (amd64 || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (windows && (386 || amd64 || arm64))

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	sqlite "modernc.org/sqlite"
)

func openDBReadOnly(dbPath string) (*sql.DB, error) {
	params := url.Values{}
	// Only set safe pragmas for read-only mode - most pragmas require write access
	params.Set("_txlock", "immediate")
	params.Set("mode", "ro")

	dsn := fmt.Sprintf("file:%s?%s", dbPath, params.Encode())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return db, nil
}

func openDB(dbPath, journalMode string) (*sql.DB, error) {
	// Set pragmas for better performance via _pragma query params.
	// Format: _pragma=name(value)
	params := url.Values{}
	for name, value := range pragmas(journalMode) {
		params.Add("_pragma", fmt.Sprintf("%s(%s)", name, value))
	}
	// Use BEGIN IMMEDIATE so writers acquire the reserved lock up front,
	// preventing deferred-to-writer upgrade deadlocks.
	params.Set("_txlock", "immediate")

	dsn := fmt.Sprintf("file:%s?%s", dbPath, params.Encode())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return db, nil
}

// SQLite result codes relevant to filesystem locking failures. This
// driver turns on extended result codes when it connects and types
// Error.Code() as an int, so the full extended value arrives intact and
// both of these are reachable. (The ncruces driver narrows Code() to
// the primary code, which is why it matches on typed constants
// instead.)
const (
	sqliteProtocol  = 15   // SQLITE_PROTOCOL: database lock protocol error.
	sqliteIOErrLock = 3850 // SQLITE_IOERR_LOCK: I/O error within file locking.
)

// isLockProtocolError reports whether err is a SQLite lock-protocol or
// lock I/O error, the signature of a filesystem (NFS, SMB, FUSE) that
// cannot support WAL's shared-memory locking.
func isLockProtocolError(err error) bool {
	if sqliteErr, ok := errors.AsType[*sqlite.Error](err); ok {
		return isLockResultCode(sqliteErr.Code())
	}
	return false
}

// isLockResultCode reports whether a SQLite result code indicates a
// filesystem locking failure.
func isLockResultCode(code int) bool {
	return code == sqliteProtocol || code == sqliteIOErrLock
}
