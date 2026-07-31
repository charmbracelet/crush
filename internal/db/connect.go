package db

import (
	"cmp"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pressly/goose/v3"
)

// pragmas returns the connection pragmas applied when opening a
// database. journalMode overrides the default WAL journal; pass "" to
// keep WAL. WAL relies on shared-memory locking that network
// filesystems (NFS, SMB) do not support, so Connect falls back to the
// rollback journal ("DELETE") when WAL locking fails.
func pragmas(journalMode string) map[string]string {
	return map[string]string{
		"foreign_keys":  "ON",
		"journal_mode":  cmp.Or(journalMode, "WAL"),
		"page_size":     "4096",
		"temp_store":    "MEMORY",
		"cache_size":    "-8000",
		"synchronous":   "NORMAL",
		"secure_delete": "ON",
		"busy_timeout":  "30000",
	}
}

var (
	gooseInitOnce sync.Once
	gooseInitErr  error
)

//go:embed migrations/*.sql
var FS embed.FS

func init() {
	goose.SetBaseFS(FS)

	if testing.Testing() {
		goose.SetLogger(goose.NopLogger())
	}
}

// connEntry holds a shared database connection, its reference count,
// and the data-directory lock that gates access to this entry. The
// lock is acquired exactly once when the entry is created and released
// when the last reference is dropped, which lets the same process open
// the same data directory concurrently while still blocking a second
// crush process from racing the storage.
type connEntry struct {
	db       *sql.DB
	refCount int
	lock     *dataDirLock
}

var (
	pool   = make(map[string]*connEntry)
	poolMu sync.Mutex
)

// ConnectOption configures a Connect call. Options are applied in
// order; later options override earlier ones for the same field.
type ConnectOption func(*connectOptions)

// connectOptions holds the resolved configuration for a Connect call.
type connectOptions struct {
	lockDataDir bool
}

// WithDataDirLock toggles acquisition of the per-data-directory lock
// for this Connect call. The lock is off by default so local-mode
// invocations do not regress today's behavior; the server's
// workspace-bootstrap path opts in. CRUSH_SKIP_DATADIR_LOCK still
// bypasses acquisition even when this option is set.
func WithDataDirLock(enable bool) ConnectOption {
	return func(o *connectOptions) { o.lockDataDir = enable }
}

// Connect opens a SQLite database connection for the given data
// directory and runs migrations. If a connection to the same database
// file already exists, the existing connection is returned with its
// reference count incremented. Callers must pair each Connect with a
// [Release] when they no longer need the connection.
func Connect(ctx context.Context, dataDir string, opts ...ConnectOption) (*sql.DB, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data.dir is not set")
	}

	var cfg connectOptions
	for _, opt := range opts {
		opt(&cfg)
	}

	dbPath := filepath.Join(dataDir, "crush.db")

	// Resolve to an absolute path so that different relative paths to
	// the same file share a single connection.
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		absPath = dbPath
	}

	poolMu.Lock()
	defer poolMu.Unlock()

	if entry, ok := pool[absPath]; ok {
		entry.refCount++
		return entry.db, nil
	}

	// Take the per-data-directory lock before opening the database so
	// we fail fast and with a clear error rather than racing another
	// crush process on the same SQLite file. The lock is released when
	// the matching Release call drops the refcount to zero. Ensuring
	// the data directory exists is required because the lock file
	// lives inside it. Locking is opt-in via WithDataDirLock so that
	// local-mode invocations do not refuse a second crush against the
	// same data dir until client/server becomes the default.
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create data directory %q: %w", dataDir, err)
	}
	var lock *dataDirLock
	if cfg.lockDataDir && !skipDataDirLock() {
		lock, err = acquireDataDirLock(dataDir)
		if err != nil {
			return nil, err
		}
	}

	releaseLock := func() {
		if lock != nil {
			lock.release()
		}
	}

	// Open with WAL first; if the filesystem cannot support WAL's
	// shared-memory locking (NFS, SMB, some FUSE mounts) SQLite either
	// fails with a lock-protocol error or silently stays on the
	// rollback journal. Either way, retry once with the rollback
	// journal, which only needs POSIX file locks.
	conn, err := openAndPing(ctx, dbPath, "")
	if isWALLockFailure(err) {
		slog.Warn("WAL journal mode unsupported on this filesystem, falling back to rollback journal",
			"path", dbPath, "error", err)
		conn, err = openAndPing(ctx, dbPath, "DELETE")
	}
	if err != nil {
		releaseLock()
		return nil, err
	}

	if err := initGoose(); err != nil {
		conn.Close()
		releaseLock()
		slog.Error("Failed to initialize goose", "error", err)
		return nil, fmt.Errorf("failed to initialize goose: %w", err)
	}

	if err := goose.Up(conn, "migrations"); err != nil {
		conn.Close()
		releaseLock()
		slog.Error("Failed to apply migrations", "error", err)
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	pool[absPath] = &connEntry{db: conn, refCount: 1, lock: lock}
	return conn, nil
}

// Release decrements the reference count for the database at the given
// data directory. When the count reaches zero the underlying connection
// is closed and removed from the pool.
func Release(dataDir string) error {
	dbPath := filepath.Join(dataDir, "crush.db")
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		absPath = dbPath
	}

	poolMu.Lock()
	defer poolMu.Unlock()

	entry, ok := pool[absPath]
	if !ok {
		return nil
	}

	entry.refCount--
	if entry.refCount > 0 {
		return nil
	}

	delete(pool, absPath)
	closeErr := entry.db.Close()
	if entry.lock != nil {
		entry.lock.release()
	}
	return closeErr
}

// ResetPool closes all pooled connections and clears the pool. This is
// intended for use in tests to ensure a clean state between test cases.
func ResetPool() {
	poolMu.Lock()
	defer poolMu.Unlock()
	for path, entry := range pool {
		entry.db.Close()
		if entry.lock != nil {
			entry.lock.release()
		}
		delete(pool, path)
	}
}

// ConnectReadOnly opens a read-only SQLite database connection without running
// migrations. Used for aggregating stats across multiple project databases.
func ConnectReadOnly(ctx context.Context, dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path is empty")
	}

	db, err := openDBReadOnly(dbPath)
	if err != nil {
		return nil, maybeLockHint(err)
	}

	db.SetMaxOpenConns(1)

	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", maybeLockHint(err))
	}

	return db, nil
}

// Journal-mode errors. SQLite answers a refused journal_mode change by
// reporting the current mode rather than failing, so these mismatches
// are detected here instead of surfacing from the driver.
var (
	// errWALUnavailable means WAL was requested but did not take
	// effect. Connect treats this as the signal to retry with the
	// rollback journal.
	errWALUnavailable = errors.New("database did not enter WAL journal mode")

	// errWALStuck means the rollback journal was requested but the
	// database stayed in WAL mode, so the fallback did not help and
	// there is nothing further to try.
	errWALStuck = errors.New("database could not leave WAL journal mode")
)

// openAndPing opens the database with the given journal mode ("" for
// the WAL default), serializes it to a single connection, verifies it
// responds to a ping, and confirms the requested journal mode took
// effect. On failure the connection is closed and a filesystem-hinting
// error is returned.
func openAndPing(ctx context.Context, dbPath, journalMode string) (*sql.DB, error) {
	conn, err := openDB(dbPath, journalMode)
	if err != nil {
		// openDB already describes the failure; only the filesystem
		// hint is missing.
		return nil, maybeLockHint(err)
	}

	// Serialize all access through a single connection. SQLite
	// serializes writes at the file level anyway, and allowing multiple
	// pool connections to interleave writes/checkpoints (especially
	// under concurrent sub-agents) has caused WAL/header desync
	// resulting in SQLITE_NOTADB (26) on the next open.
	conn.SetMaxOpenConns(1)

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", maybeLockHint(err))
	}

	if err := verifyJournalMode(ctx, conn, journalMode); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", maybeLockHint(err))
	}
	return conn, nil
}

// verifyJournalMode confirms the requested journal mode took effect.
// SQLite reports the current mode instead of failing when it refuses a
// journal_mode change, so both directions are checked: WAL not applying
// means the filesystem cannot support it, and the rollback journal not
// applying means the fallback has nowhere left to go.
func verifyJournalMode(ctx context.Context, conn *sql.DB, journalMode string) error {
	want := pragmas(journalMode)["journal_mode"]

	var got string
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&got); err != nil {
		return err
	}
	if strings.EqualFold(want, got) {
		return nil
	}

	if strings.EqualFold(want, "wal") {
		return errWALUnavailable
	}
	// Requested the rollback journal and stayed in WAL. Converting away
	// from WAL needs to read the existing write-ahead log, which itself
	// needs the shared-memory locking we are trying to avoid, so this is
	// the end of the line rather than another retry.
	if strings.EqualFold(got, "wal") {
		return errWALStuck
	}
	return fmt.Errorf("journal mode %q was requested but the database is in %q", want, got)
}

// isWALLockFailure reports whether err means WAL could not be used
// because of filesystem locking, either because the driver said so or
// because the journal_mode change quietly did not take. It is worth
// retrying with the rollback journal in both cases.
func isWALLockFailure(err error) bool {
	return errors.Is(err, errWALUnavailable) || isLockProtocolError(err)
}

// maybeLockHint appends advice to failures that point at the filesystem
// under the data directory, so users understand the cause and the
// data_directory workaround. Other errors pass through untouched.
func maybeLockHint(err error) error {
	const relocate = "set options.data_directory to a local path"

	switch {
	case errors.Is(err, errWALStuck):
		// The database is already in WAL and cannot be converted here,
		// so relocating alone is not enough; the existing file has to
		// come along or be checkpointed from a machine that can.
		return fmt.Errorf("%w; the existing database cannot be converted on this filesystem, so copy the data directory to local storage and %s", err, relocate)
	case isWALLockFailure(err):
		// Shared-memory locking is the usual culprit, and network
		// filesystems are the usual reason it is missing, but it is not
		// the only one, so suggest rather than diagnose.
		return fmt.Errorf("%w; SQLite could not use the locking this filesystem provides, which is common on network mounts such as NFS or SMB. If the data directory is on one, %s", err, relocate)
	default:
		return err
	}
}

func initGoose() error {
	gooseInitOnce.Do(func() {
		goose.SetBaseFS(FS)
		gooseInitErr = goose.SetDialect("sqlite3")
	})

	return gooseInitErr
}
