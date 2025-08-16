package db

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

// SetupTestDB creates an in-memory SQLite database with all migrations applied.
// It returns a clean database connection that will be automatically closed
// when the test completes.
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Create in-memory SQLite database
	conn, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Verify connection
	require.NoError(t, conn.PingContext(context.Background()))

	// Set essential pragmas for testing
	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = MEMORY;", // Faster for testing
		"PRAGMA synchronous = OFF;",     // Faster for testing
	}

	for _, pragma := range pragmas {
		_, err = conn.ExecContext(context.Background(), pragma)
		require.NoError(t, err)
	}

	// Set up goose with embedded migrations
	goose.SetBaseFS(FS)
	require.NoError(t, goose.SetDialect("sqlite3"))

	// Apply all migrations
	err = goose.Up(conn, "migrations")
	require.NoError(t, err)

	// Register cleanup to close the database
	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

// SetupTestDBWithData creates a test database and allows custom data setup.
// The setupFunc will be called after migrations are applied.
func SetupTestDBWithData(t *testing.T, setupFunc func(*sql.DB)) *sql.DB {
	t.Helper()

	conn := SetupTestDB(t)
	if setupFunc != nil {
		setupFunc(conn)
	}
	return conn
}

// CreateTestSession creates a basic test session in the database.
// This is a helper for tests that need session data.
func CreateTestSession(conn *sql.DB, sessionID, title string) error {
	_, err := conn.ExecContext(context.Background(), `
		INSERT INTO sessions (id, title, message_count, prompt_tokens, completion_tokens, cost, created_at, updated_at) 
		VALUES (?, ?, 0, 0, 0, 0.0, ?, ?)
	`, sessionID, title, 1000, 1000)
	return err
}
