package agent

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/stretchr/testify/require"
)

func TestIsRetriableError(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()
		require.False(t, isRetriableError(nil))
	})

	t.Run("429 rate limit", func(t *testing.T) {
		t.Parallel()
		require.True(t, isRetriableError(&fantasy.ProviderError{
			StatusCode: 429,
			Title:      "rate limit",
			Message:    "too many requests",
		}))
	})

	t.Run("503 service unavailable", func(t *testing.T) {
		t.Parallel()
		require.True(t, isRetriableError(&fantasy.ProviderError{
			StatusCode: 503,
			Title:      "service unavailable",
			Message:    "temporarily unavailable",
		}))
	})

	t.Run("502 bad gateway", func(t *testing.T) {
		t.Parallel()
		require.True(t, isRetriableError(&fantasy.ProviderError{
			StatusCode: 502,
			Title:      "bad gateway",
			Message:    "bad gateway",
		}))
	})

	t.Run("504 gateway timeout", func(t *testing.T) {
		t.Parallel()
		require.True(t, isRetriableError(&fantasy.ProviderError{
			StatusCode: 504,
			Title:      "gateway timeout",
			Message:    "gateway timeout",
		}))
	})

	t.Run("stream idle timeout is retriable", func(t *testing.T) {
		t.Parallel()
		err := &fantasy.ProviderError{
			Title:   "network error",
			Message: streamIdleTimeoutMessage(),
			Cause:   errors.New("stream idle timeout"),
		}
		require.True(t, isRetriableError(err),
			"stream idle timeout must be retriable so mid-run failures can recover")
	})

	t.Run("overloaded message is retriable", func(t *testing.T) {
		t.Parallel()
		require.True(t, isRetriableError(&fantasy.ProviderError{
			StatusCode: 529,
			Message:    "Overloaded",
		}))
	})

	t.Run("accounts exhausted message is retriable", func(t *testing.T) {
		t.Parallel()
		// Mid-stream SSE error from copilot-api when all accounts are rate limited
		require.True(t, isRetriableError(&fantasy.ProviderError{
			StatusCode: 0, // No HTTP status code for mid-stream errors
			Message:    `{"message":"All accounts exhausted","type":"error"}`,
		}))
	})

	t.Run("accounts exhausted plain error is retriable", func(t *testing.T) {
		t.Parallel()
		require.True(t, isRetriableError(errors.New("received error while streaming: {\"message\":\"All accounts exhausted\",\"type\":\"rate_limit_error\"}")))
	})

	t.Run("400 bad request is not retriable", func(t *testing.T) {
		t.Parallel()
		require.False(t, isRetriableError(&fantasy.ProviderError{
			StatusCode: 400,
			Title:      "bad request",
			Message:    "invalid input",
		}))
	})

	t.Run("plain timeout error is retriable", func(t *testing.T) {
		t.Parallel()
		require.True(t, isRetriableError(errors.New("request timeout")))
	})

	t.Run("unrelated error is not retriable", func(t *testing.T) {
		t.Parallel()
		require.False(t, isRetriableError(errors.New("permission denied")))
	})
}

func TestRetryDelay(t *testing.T) {
	t.Parallel()

	// retryDelay includes ±25% jitter, so check that the result
	// falls within [0.75*base, 1.25*base] for each attempt.
	assertInRange := func(t *testing.T, base time.Duration, got time.Duration) {
		t.Helper()
		lo := time.Duration(float64(base) * 0.75)
		hi := time.Duration(float64(base) * 1.25)
		require.GreaterOrEqual(t, got, lo, "delay %v below lower bound %v", got, lo)
		require.LessOrEqual(t, got, hi, "delay %v above upper bound %v", got, hi)
	}

	assertInRange(t, retryBaseDelay, retryDelay(1, 0))   // ~3s
	assertInRange(t, retryBaseDelay*2, retryDelay(2, 0)) // ~6s
	assertInRange(t, retryBaseDelay*4, retryDelay(3, 0)) // ~12s
	assertInRange(t, retryBaseDelay*8, retryDelay(4, 0)) // ~24s
	assertInRange(t, retryMaxDelay, retryDelay(5, 0))    // ~48s
	assertInRange(t, retryMaxDelay, retryDelay(10, 0))   // clamped at ~48s
}

func TestAddJitter(t *testing.T) {
	t.Parallel()

	const base = 10 * time.Second
	for range 100 {
		got := addJitter(base)
		lo := time.Duration(float64(base) * 0.75)
		hi := time.Duration(float64(base) * 1.25)
		require.GreaterOrEqual(t, got, lo)
		require.LessOrEqual(t, got, hi)
	}
}

func TestRetryDelayProducesSpread(t *testing.T) {
	t.Parallel()

	// Call retryDelay many times for the same attempt and verify that
	// not all values are identical — i.e. jitter is actually applied.
	seen := make(map[time.Duration]struct{})
	for range 50 {
		seen[retryDelay(1, 0)] = struct{}{}
	}
	require.Greater(t, len(seen), 1,
		"retryDelay should produce varied values due to jitter")
}

func TestRetryDelayRespectsRetryAfter(t *testing.T) {
	t.Parallel()

	t.Run("retry-after larger than backoff wins", func(t *testing.T) {
		t.Parallel()
		serverDelay := 60 * time.Second
		got := retryDelay(1, serverDelay)
		require.Equal(t, serverDelay, got,
			"when Retry-After exceeds exponential backoff, the server value should be used")
	})

	t.Run("backoff larger than retry-after wins", func(t *testing.T) {
		t.Parallel()
		serverDelay := 1 * time.Second
		got := retryDelay(5, serverDelay)
		lo := time.Duration(float64(retryMaxDelay) * 0.75)
		require.GreaterOrEqual(t, got, lo,
			"when backoff exceeds Retry-After, exponential backoff should be used")
	})

	t.Run("zero retry-after falls back to backoff", func(t *testing.T) {
		t.Parallel()
		got := retryDelay(2, 0)
		lo := time.Duration(float64(retryBaseDelay*2) * 0.75)
		hi := time.Duration(float64(retryBaseDelay*2) * 1.25)
		require.GreaterOrEqual(t, got, lo)
		require.LessOrEqual(t, got, hi)
	})
}

func TestRetryAfterFromError(t *testing.T) {
	t.Parallel()

	t.Run("unwraps RetryAfterError", func(t *testing.T) {
		t.Parallel()
		inner := &fantasy.ProviderError{StatusCode: 429, Message: "rate limited"}
		err := &hyper.RetryAfterError{Err: inner, After: 30 * time.Second}
		require.Equal(t, 30*time.Second, retryAfterFromError(err))
	})

	t.Run("unwraps nested RetryAfterError", func(t *testing.T) {
		t.Parallel()
		inner := &fantasy.ProviderError{StatusCode: 429, Message: "rate limited"}
		raErr := &hyper.RetryAfterError{Err: inner, After: 15 * time.Second}
		wrapped := fmt.Errorf("stream failed: %w", raErr)
		require.Equal(t, 15*time.Second, retryAfterFromError(wrapped))
	})

	t.Run("returns zero for plain error", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, time.Duration(0), retryAfterFromError(errors.New("something")))
	})

	t.Run("returns zero for nil", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, time.Duration(0), retryAfterFromError(nil))
	})

	t.Run("returns zero for ProviderError without RetryAfter", func(t *testing.T) {
		t.Parallel()
		err := &fantasy.ProviderError{StatusCode: 429, Message: "rate limited"}
		require.Equal(t, time.Duration(0), retryAfterFromError(err))
	})
}

func TestRetryAfterErrorUnwrapsToProviderError(t *testing.T) {
	t.Parallel()

	inner := &fantasy.ProviderError{StatusCode: 429, Message: "rate limited"}
	err := &hyper.RetryAfterError{Err: inner, After: 10 * time.Second}

	var providerErr *fantasy.ProviderError
	require.True(t, errors.As(err, &providerErr), "RetryAfterError must unwrap to ProviderError")
	require.Equal(t, 429, providerErr.StatusCode)
	require.True(t, isRetriableError(err), "RetryAfterError wrapping 429 must be retriable")
}
