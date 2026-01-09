//go:build !((darwin && (amd64 || arm64)) || (freebsd && (amd64 || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (windows && (386 || amd64 || arm64)))

package db

import (
	"database/sql"
	"fmt"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func openDB(dbPath string) (*sql.DB, error) {
	// Set pragmas for better performance.
	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA page_size = 4096;",
		"PRAGMA cache_size = -8000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA secure_delete = ON;",
		"PRAGMA busy_timeout = 5000;",
	}

	db, err := driver.Open(dbPath, func(c *sqlite3.Conn) error {
		for _, pragma := range pragmas {
			if err := c.Exec(pragma); err != nil {
				return fmt.Errorf("failed to set pragma %q: %w", pragma, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return db, nil
}
