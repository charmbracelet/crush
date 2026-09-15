package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// OpenDBFile opens (creating if necessary) a SQLite database at an
// arbitrary path using the same driver and pragmas as the main
// crush.db. Unlike Connect it is not pooled, data-dir locked, or
// migrated — the caller owns the lifecycle and the schema. Intended
// for disposable sidecar databases such as the project index, where
// deleting the file must always be safe.
func OpenDBFile(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}
	conn, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}
	// Same rule as Connect: a single connection serializes access —
	// interleaved multi-conn writes have caused WAL/header desync
	// (SQLITE_NOTADB) under concurrent sub-agents.
	conn.SetMaxOpenConns(1)
	return conn, nil
}
