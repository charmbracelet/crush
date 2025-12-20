package ratelimit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	// TokenLimit is the maximum number of tokens allowed per window.
	// This includes both input and output tokens.
	TokenLimit = 3500 // ~45 messages worth of tokens (adjust as needed)
	// WindowDuration is the duration of the rate limit window.
	WindowDuration = 5 * time.Hour
)

var (
	// ErrRateLimitExceeded is returned when the rate limit is exceeded.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// Limiter handles rate limiting for message creation.
type Limiter struct {
	db *sql.DB
}

// New creates a new rate limiter.
func New(db *sql.DB) *Limiter {
	return &Limiter{db: db}
}

// Check verifies if tokens can be used without exceeding the rate limit.
// It returns an error if the limit would be exceeded.
func (l *Limiter) Check(ctx context.Context, inputTokens, outputTokens int) error {
	totalTokens := inputTokens + outputTokens
	if totalTokens < 0 {
		totalTokens = 0
	}

	usedTokens, windowStart, err := l.getCurrentUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to check rate limit: %w", err)
	}

	if usedTokens+totalTokens > TokenLimit {
		timeRemaining := time.Until(windowStart.Add(WindowDuration))
		return &RateLimitError{
			Limit:         TokenLimit,
			Current:       usedTokens,
			WindowStart:   windowStart,
			TimeRemaining: timeRemaining,
		}
	}

	return nil
}

// RecordTokens records tokens used in the current rate limit window.
func (l *Limiter) RecordTokens(ctx context.Context, inputTokens, outputTokens int) error {
	totalTokens := inputTokens + outputTokens
	if totalTokens <= 0 {
		return nil
	}

	now := time.Now()
	windowStart := l.getWindowStart(now)

	// Insert or update the current window's token count.
	query := `
		INSERT INTO rate_limit_usage (window_start, message_count, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT(window_start) DO UPDATE SET
			message_count = message_count + ?
	`

	_, err := l.db.ExecContext(ctx, query, windowStart.Unix(), totalTokens, now.Unix(), totalTokens)
	if err != nil {
		return fmt.Errorf("failed to record tokens: %w", err)
	}

	return nil
}

// RecordMessage records a new message in the current rate limit window.
// This method is deprecated - use RecordTokens with actual token counts.
// It estimates 1 token per character as a rough approximation.
func (l *Limiter) RecordMessage(ctx context.Context) error {
	return l.RecordTokens(ctx, 100, 100) // Estimate 200 tokens per message
}

// Check verifies if a new message can be created without exceeding the rate limit.
// This method is deprecated - use Check with actual token counts.
// It estimates 1 token per character as a rough approximation.
func (l *Limiter) CheckMessage(ctx context.Context) error {
	return l.Check(ctx, 100, 100) // Estimate 200 tokens per message
}

// getCurrentUsage returns the current message count and window start time.
func (l *Limiter) getCurrentUsage(ctx context.Context) (int, time.Time, error) {
	now := time.Now()
	windowStart := l.getWindowStart(now)

	var count int
	query := `
		SELECT COALESCE(SUM(message_count), 0)
		FROM rate_limit_usage
		WHERE window_start >= ?
	`

	err := l.db.QueryRowContext(ctx, query, windowStart.Unix()).Scan(&count)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, time.Time{}, fmt.Errorf("failed to get current usage: %w", err)
	}

	return count, windowStart, nil
}

// getWindowStart calculates the start of the current rate limit window.
func (l *Limiter) getWindowStart(now time.Time) time.Time {
	// Calculate how many complete windows have passed since Unix epoch.
	windowDurationSeconds := int64(WindowDuration.Seconds())
	currentWindowIndex := now.Unix() / windowDurationSeconds
	windowStartUnix := currentWindowIndex * windowDurationSeconds

	return time.Unix(windowStartUnix, 0)
}

// CleanupOldWindows removes rate limit windows older than the window duration.
func (l *Limiter) CleanupOldWindows(ctx context.Context) error {
	now := time.Now()
	cutoff := now.Add(-WindowDuration).Unix()

	query := `DELETE FROM rate_limit_usage WHERE window_start < ?`
	_, err := l.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old windows: %w", err)
	}

	return nil
}

// GetStatus returns the current rate limit status.
func (l *Limiter) GetStatus(ctx context.Context) (*Status, error) {
	count, windowStart, err := l.getCurrentUsage(ctx)
	if err != nil {
		return nil, err
	}

	remaining := TokenLimit - count
	if remaining < 0 {
		remaining = 0
	}

	windowEnd := windowStart.Add(WindowDuration)
	timeUntilReset := time.Until(windowEnd)

	return &Status{
		Limit:          TokenLimit,
		Used:           count,
		Remaining:      remaining,
		WindowStart:    windowStart,
		WindowEnd:      windowEnd,
		TimeUntilReset: timeUntilReset,
	}, nil
}

// Status represents the current rate limit status.
type Status struct {
	Limit          int           // Maximum tokens allowed.
	Used           int           // Tokens used in current window.
	Remaining      int           // Tokens remaining in current window.
	WindowStart    time.Time     // Start of current window.
	WindowEnd      time.Time     // End of current window.
	TimeUntilReset time.Duration // Time until the window resets.
}

// RateLimitError is returned when the rate limit is exceeded.
type RateLimitError struct {
	Limit         int           // Maximum tokens allowed.
	Current       int           // Current token count.
	WindowStart   time.Time     // Start of current window.
	TimeRemaining time.Duration // Time until the window resets.
}

func (e *RateLimitError) Error() string {
	hours := int(e.TimeRemaining.Hours())
	minutes := int(e.TimeRemaining.Minutes()) % 60

	var timeStr string
	if hours > 0 {
		timeStr = fmt.Sprintf("%d hour(s) and %d minute(s)", hours, minutes)
	} else {
		timeStr = fmt.Sprintf("%d minute(s)", minutes)
	}

	return fmt.Sprintf(
		"rate limit exceeded: %d/%d tokens used in current 5-hour window. Please try again in %s",
		e.Current,
		e.Limit,
		timeStr,
	)
}

// IsRateLimitError checks if an error is a rate limit error.
func IsRateLimitError(err error) bool {
	var rle *RateLimitError
	return errors.As(err, &rle)
}
