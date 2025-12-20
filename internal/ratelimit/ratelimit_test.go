package ratelimit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create the rate_limit_usage table.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS rate_limit_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			window_start INTEGER NOT NULL UNIQUE,
			message_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
		)
	`)
	require.NoError(t, err)

	return db
}

func TestLimiter_Check(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	limiter := New(db)
	ctx := context.Background()

	// First check should succeed (no tokens used yet).
	err := limiter.Check(ctx, 10, 0)
	require.NoError(t, err)

	// Record 10 tokens.
	err = limiter.RecordTokens(ctx, 5, 5)
	require.NoError(t, err)

	// Check should still succeed with 10 more tokens.
	err = limiter.Check(ctx, 10, 0)
	require.NoError(t, err)

	// Record those 10 tokens.
	err = limiter.RecordTokens(ctx, 5, 5)
	require.NoError(t, err)

	// Check should still succeed with 20 more tokens.
	err = limiter.Check(ctx, 20, 0)
	require.NoError(t, err)

	// Record those 20 tokens to reach 40 total.
	err = limiter.RecordTokens(ctx, 10, 10)
	require.NoError(t, err)

	// Check should still succeed with 10 more tokens to reach exactly 50.
	err = limiter.Check(ctx, 10, 0)
	require.NoError(t, err)

	// Record the final 10 tokens to reach exactly the limit.
	err = limiter.RecordTokens(ctx, 5, 5)
	require.NoError(t, err)

	// Now check should fail with even 1 token.
	err = limiter.Check(ctx, 1, 0)
	require.Error(t, err)
	require.True(t, IsRateLimitError(err))

	var rle *RateLimitError
	require.ErrorAs(t, err, &rle)
	require.Equal(t, TokenLimit, rle.Limit)
	require.True(t, rle.Current >= TokenLimit)
}

func TestLimiter_GetStatus(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	limiter := New(db)
	ctx := context.Background()

	// Initial status should show no usage.
	status, err := limiter.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, TokenLimit, status.Limit)
	require.Equal(t, 0, status.Used)
	require.Equal(t, TokenLimit, status.Remaining)

	// Record 25 tokens.
	err = limiter.RecordTokens(ctx, 12, 13)
	require.NoError(t, err)

	// Status should reflect 25 tokens used.
	status, err = limiter.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, TokenLimit, status.Limit)
	require.Equal(t, 25, status.Used)
	require.Equal(t, TokenLimit-25, status.Remaining)
}

func TestLimiter_RecordTokens(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	limiter := New(db)
	ctx := context.Background()

	// Record 10 tokens.
	err := limiter.RecordTokens(ctx, 5, 5)
	require.NoError(t, err)

	// Verify the tokens were recorded.
	status, err := limiter.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, 10, status.Used)

	// Record another 20 tokens.
	err = limiter.RecordTokens(ctx, 10, 10)
	require.NoError(t, err)

	// Verify the count increased.
	status, err = limiter.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, 30, status.Used)
}

func TestLimiter_CleanupOldWindows(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	defer db.Close()

	limiter := New(db)
	ctx := context.Background()

	// Insert an old window (more than 5 hours ago).
	oldWindowStart := time.Now().Add(-6 * time.Hour).Unix()
	_, err := db.Exec(
		"INSERT INTO rate_limit_usage (window_start, message_count, created_at) VALUES (?, ?, ?)",
		oldWindowStart,
		10,
		time.Now().Unix(),
	)
	require.NoError(t, err)

	// Record tokens in the current window.
	err = limiter.RecordTokens(ctx, 5, 5)
	require.NoError(t, err)

	// Cleanup old windows.
	err = limiter.CleanupOldWindows(ctx)
	require.NoError(t, err)

	// Verify the old window was deleted.
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM rate_limit_usage WHERE window_start = ?", oldWindowStart).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Verify the current window is still there.
	status, err := limiter.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, 10, status.Used)
}

func TestRateLimitError_Error(t *testing.T) {
	t.Parallel()

	err := &RateLimitError{
		Limit:         50,
		Current:       50,
		WindowStart:   time.Now().Add(-3 * time.Hour),
		TimeRemaining: 2*time.Hour + 15*time.Minute,
	}

	errorMsg := err.Error()
	require.Contains(t, errorMsg, "50/50")
	require.Contains(t, errorMsg, "2 hour(s) and 15 minute(s)")
}
