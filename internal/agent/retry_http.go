package agent

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/hyper"
)

// retryAfterFromError extracts the Retry-After duration from an error chain.
// Returns zero if the error does not carry retry-after information.
func retryAfterFromError(err error) time.Duration {
	var ra *hyper.RetryAfterError
	if errors.As(err, &ra) {
		return ra.After
	}
	return 0
}

const (
	// maxRetriableAttempts is the maximum number of retry attempts for
	// transient errors (429 rate limit, 503 service unavailable).
	maxRetriableAttempts = 5

	// retryBaseDelay is the base delay for exponential backoff.
	// Retry schedule: 3s, 6s, 12s, 24s, 48s.
	retryBaseDelay = 3 * time.Second

	// retryMaxDelay is the maximum delay between retries.
	retryMaxDelay = 48 * time.Second
)

// isRetriableError reports whether the error should be retried with exponential
// backoff. Uses a "default retriable" strategy: only explicitly non-retriable
// errors return false, everything else is retriable.
//
// Non-retriable errors include:
//   - 400 Bad Request (malformed request)
//   - 401 Unauthorized (authentication failed)
//   - 403 Forbidden (permission denied)
//   - 404 Not Found (resource doesn't exist)
//   - Context cancellation (user aborted)
type nonRetriableError struct {
	err error
}

func (e *nonRetriableError) Error() string {
	return e.err.Error()
}

func (e *nonRetriableError) Unwrap() error {
	return e.err
}

func markNonRetriableError(err error) error {
	if err == nil {
		return nil
	}
	return &nonRetriableError{err: err}
}

func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	var nonRetriable *nonRetriableError
	if errors.As(err, &nonRetriable) {
		return false
	}

	if isToolResultProtocolMismatchError(err.Error()) {
		return true
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var providerErr *fantasy.ProviderError
	if errors.As(err, &providerErr) {
		if isToolResultProtocolMismatchError(providerErr.Message) {
			return true
		}
		if providerErr.StatusCode >= 400 && providerErr.StatusCode < 500 {
			if providerErr.StatusCode == 429 {
				return true
			}
			return false
		}
		return true
	}

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "status: 400") ||
		strings.Contains(errStr, "status: 401") ||
		strings.Contains(errStr, "status: 403") ||
		strings.Contains(errStr, "status: 404") ||
		strings.Contains(errStr, "http 400") ||
		strings.Contains(errStr, "http 401") ||
		strings.Contains(errStr, "http 403") ||
		strings.Contains(errStr, "http 404") ||
		strings.Contains(errStr, "bad request") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "forbidden") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "context canceled") ||
		strings.Contains(errStr, "context deadline exceeded") {
		return false
	}

	return true
}

// retryDelay calculates the delay for the given attempt number using
// exponential backoff with jitter: base * 2^(attempt-1) ± 25%.
// The jitter prevents thundering-herd collisions when multiple
// concurrent subagents all retry against the same rate-limited API.
// With base=3s: ~3s, ~6s, ~12s, ~24s, ~48s (each ±25%).
//
// If serverRetryAfter is positive (from a Retry-After header), the
// returned delay is max(exponentialBackoff, serverRetryAfter).
func isToolResultProtocolMismatchError(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "no tool call found for function call output") {
		return true
	}
	if strings.Contains(msg, "no tool call found for tool call output") {
		return true
	}
	if strings.Contains(msg, "unexpected `tool_use_id` found in `tool_result` blocks") {
		return true
	}
	if strings.Contains(msg, "unexpected tool_use_id found in tool_result blocks") {
		return true
	}
	return false
}

func retryDelay(attempt int, serverRetryAfter time.Duration) time.Duration {
	delay := retryBaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay > retryMaxDelay {
			delay = retryMaxDelay
			break
		}
	}
	backoff := addJitter(delay)
	if serverRetryAfter > backoff {
		return serverRetryAfter
	}
	return backoff
}

func waitForRetryDelay(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// addJitter adds ±25% random jitter to a duration so that concurrent
// retries from parallel subagents do not all fire at the same instant.
func addJitter(d time.Duration) time.Duration {
	// jitter range: [0.75*d, 1.25*d]
	jitter := float64(d) * (0.75 + rand.Float64()*0.5) //nolint:gosec
	return time.Duration(jitter)
}
