//go:build !openbsd && !netbsd && !android

package db

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

func openDB(dbPath string) (*sql.DB, error) {
	// Set pragmas for better performance via _pragma query params.
	// Format: _pragma=name(value)
	params := url.Values{}
	params.Add("_pragma", "foreign_keys(on)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "page_size(4096)")
	params.Add("_pragma", "cache_size(-8000)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_pragma", "secure_delete(on)")

	dsn := fmt.Sprintf("file:%s?%s", dbPath, params.Encode())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return db, nil
}
