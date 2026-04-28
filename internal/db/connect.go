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

//go:embed migrations/*.sql
var FS embed.FS

func init() {
	goose.SetBaseFS(FS)

	if testing.Testing() {
		goose.SetLogger(goose.NopLogger())
	}
}

// Connect opens a SQLite database connection and runs migrations.
func Connect(ctx context.Context, dataDir string) (*sql.DB, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data.dir is not set")
	}
	dbPath := filepath.Join(dataDir, "crush.db")

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

	return db, nil
}

func initGoose() error {
	gooseInitOnce.Do(func() {
		goose.SetBaseFS(FS)
		gooseInitErr = goose.SetDialect("sqlite3")
	})

	return gooseInitErr
}
