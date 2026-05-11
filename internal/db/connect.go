package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"

	"github.com/pressly/goose/v3"
)

var (
	pragmas = map[string]string{
		"foreign_keys":  "ON",
		"journal_mode":  "WAL",
		"page_size":     "4096",
		"cache_size":    "-8000",
		"synchronous":   "NORMAL",
		"secure_delete": "ON",
		"busy_timeout":  "30000",
	}
	gooseInitOnce sync.Once
	gooseInitErr  error
)

// connEntry holds a pooled database connection with reference counting.
type connEntry struct {
	db       *sql.DB
	refCount int
}

var (
	poolMu sync.Mutex
	pool   = make(map[string]*connEntry)
)

//go:embed migrations/*.sql
var FS embed.FS

func init() {
	goose.SetBaseFS(FS)

	if testing.Testing() {
		goose.SetLogger(goose.NopLogger())
	}
}

// Connect opens a SQLite database connection and runs migrations.
// Multiple calls with the same dataDir return the same *sql.DB and increment
// a reference count. Call Release when done to decrement the count; the
// connection is closed when the last reference is released.
func Connect(ctx context.Context, dataDir string) (*sql.DB, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data.dir is not set")
	}

	// Resolve to absolute path so different relative paths to the same file
	// share a single connection.
	absDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve data dir: %w", err)
	}

	poolMu.Lock()
	defer poolMu.Unlock()

	// Return existing connection if already open.
	if entry, ok := pool[absDir]; ok {
		entry.refCount++
		return entry.db, nil
	}

	dbPath := filepath.Join(absDir, "crush.db")

	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}

	// Serialize all access through a single connection. SQLite serializes
	// writes at the file level anyway, and allowing multiple pool
	// connections to interleave writes/checkpoints (especially under
	// concurrent sub-agents) has caused WAL/header desync resulting in
	// SQLITE_NOTADB (26) on the next open.
	db.SetMaxOpenConns(1)

	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := initGoose(); err != nil {
		slog.Error("Failed to initialize goose", "error", err)
		return nil, fmt.Errorf("failed to initialize goose: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		slog.Error("Failed to apply migrations", "error", err)
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	pool[absDir] = &connEntry{db: db, refCount: 1}
	return db, nil
}

// Release decrements the reference count for the connection associated with
// dataDir. When the count reaches zero, the connection is closed and removed
// from the pool.
func Release(dataDir string) {
	absDir, err := filepath.Abs(dataDir)
	if err != nil {
		return
	}

	poolMu.Lock()
	defer poolMu.Unlock()

	entry, ok := pool[absDir]
	if !ok {
		return
	}

	entry.refCount--
	if entry.refCount <= 0 {
		entry.db.Close()
		delete(pool, absDir)
	}
}

// ResetPool closes all connections and clears the pool. For testing only.
func ResetPool() {
	poolMu.Lock()
	defer poolMu.Unlock()

	for path, entry := range pool {
		entry.db.Close()
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
