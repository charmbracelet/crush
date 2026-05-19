package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/pressly/goose/v3"
)

// pragma is a single SQLite pragma with a guaranteed execution order.
type pragma struct {
	name  string
	value string
}

// basePragmas are applied in order on every connection. page_size must
// come before journal_mode because setting WAL writes to the database
// and locks in the current page size on new databases. temp_store=MEMORY
// keeps SQLite temp files in RAM instead of the OS temp directory,
// which avoids SQLITE_CANTOPEN errors in sandboxed environments
// (e.g. Landlock) that restrict access to /tmp.
var basePragmas = []pragma{
	{"page_size", "4096"},
	{"temp_store", "MEMORY"},
	{"journal_mode", "WAL"},
	{"foreign_keys", "ON"},
	{"cache_size", "-8000"},
	{"synchronous", "NORMAL"},
	{"secure_delete", "ON"},
	{"busy_timeout", "30000"},
}

var (
	gooseInitOnce sync.Once
	gooseInitErr  error

	// mmapAutoEnabled is set to true when Connect auto-detects a
	// blocked mmap and enables exclusive locking mode automatically.
	// Callers (e.g. the TUI) can check this via [MmapAutoEnabled]
	// to show a warning banner.
	mmapAutoEnabled bool
	mmapFlagMu      sync.RWMutex
)

// MmapAutoEnabled returns true if mmap was auto-detected as blocked
// and exclusive locking was enabled automatically.
func MmapAutoEnabled() bool {
	mmapFlagMu.RLock()
	defer mmapFlagMu.RUnlock()
	return mmapAutoEnabled
}

//go:embed migrations/*.sql
var FS embed.FS

func init() {
	goose.SetBaseFS(FS)

	if testing.Testing() {
		goose.SetLogger(goose.NopLogger())
	}
}

// connEntry holds a shared database connection and its reference count.
type connEntry struct {
	db       *sql.DB
	refCount int
	lockFile *os.File // exclusive-mode lockfile; nil when not in exclusive mode.
}

var (
	pool   = make(map[string]*connEntry)
	poolMu sync.Mutex
)

// ConnectOptions configures optional behavior for [Connect].
type ConnectOptions struct {
	// ExclusiveLock uses PRAGMA locking_mode=EXCLUSIVE, which
	// eliminates the shared-memory (-shm) file and avoids mmap.
	// This fixes "unable to open database" errors in sandboxed
	// environments that restrict mmap, but only one process can
	// access the database at a time.
	ExclusiveLock bool
}

// Connect opens a SQLite database connection for the given data
// directory and runs migrations. If a connection to the same database
// file already exists, the existing connection is returned with its
// reference count incremented. Callers must pair each Connect with a
// [Release] when they no longer need the connection.
func Connect(ctx context.Context, dataDir string, opts ...ConnectOptions) (*sql.DB, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data.dir is not set")
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

	var o ConnectOptions
	if len(opts) > 0 {
		o = opts[0]
	}

	pragmaList := basePragmas
	exclusiveMode := o.ExclusiveLock
	if o.ExclusiveLock {
		pragmaList = append(slices.Clone(basePragmas), pragma{"locking_mode", "EXCLUSIVE"})
	} else if !mmapAvailable(probeMmapDir(dbPath)) {
		pragmaList = append(slices.Clone(basePragmas), pragma{"locking_mode", "EXCLUSIVE"})
		exclusiveMode = true
		slog.Warn("Mmap appears blocked in this environment; enabled exclusive SQLite locking mode automatically.")
		mmapFlagMu.Lock()
		mmapAutoEnabled = true
		mmapFlagMu.Unlock()
	}

	// In exclusive mode, acquire a cross-process advisory lock so a
	// second Crush instance gets a clear error instead of a cryptic
	// SQLite failure.
	var lockFile *os.File
	if exclusiveMode {
		lockPath := filepath.Join(dataDir, "crush.lock")
		lockFile, err = tryExclusiveLock(lockPath)
		if err != nil {
			return nil, err
		}
	}

	conn, err := openDB(dbPath, pragmaList)
	if err != nil {
		if lockFile != nil {
			lockFile.Close()
		}
		return nil, err
	}

	// Serialize all access through a single connection. SQLite
	// serializes writes at the file level anyway, and allowing multiple
	// pool connections to interleave writes/checkpoints (especially
	// under concurrent sub-agents) has caused WAL/header desync
	// resulting in SQLITE_NOTADB (26) on the next open.
	conn.SetMaxOpenConns(1)

	if err = conn.PingContext(ctx); err != nil {
		conn.Close()
		if lockFile != nil {
			lockFile.Close()
		}
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := initGoose(); err != nil {
		conn.Close()
		if lockFile != nil {
			lockFile.Close()
		}
		slog.Error("Failed to initialize goose", "error", err)
		return nil, fmt.Errorf("failed to initialize goose: %w", err)
	}

	if err := goose.Up(conn, "migrations"); err != nil {
		conn.Close()
		if lockFile != nil {
			lockFile.Close()
		}
		slog.Error("Failed to apply migrations", "error", err)
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	pool[absPath] = &connEntry{db: conn, refCount: 1, lockFile: lockFile}
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

	// Checkpoint and truncate the WAL before closing so the -wal and
	// -shm sidecar files are cleaned up. This reduces the chance of
	// stale sidecar files causing issues on the next open. Errors
	// are best-effort — we still close the connection regardless.
	if _, err := entry.db.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		slog.Warn("WAL checkpoint failed during release", "error", err)
	}

	dbErr := entry.db.Close()
	if entry.lockFile != nil {
		entry.lockFile.Close()
	}
	return dbErr
}

// ResetPool closes all pooled connections and clears the pool. This is
// intended for use in tests to ensure a clean state between test cases.
func ResetPool() {
	poolMu.Lock()
	defer poolMu.Unlock()
	for path, entry := range pool {
		entry.db.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)") //nolint:errcheck
		entry.db.Close()
		if entry.lockFile != nil {
			entry.lockFile.Close()
		}
		delete(pool, path)
	}
}

func initGoose() error {
	gooseInitOnce.Do(func() {
		goose.SetBaseFS(FS)
		gooseInitErr = goose.SetDialect("sqlite3")
	})

	return gooseInitErr
}
