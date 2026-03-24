package agent

import (
	"errors"
	"testing"
	"time"

	"charm.land/fantasy"
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

	assertInRange(t, retryBaseDelay, retryDelay(1))   // ~3s
	assertInRange(t, retryBaseDelay*2, retryDelay(2)) // ~6s
	assertInRange(t, retryBaseDelay*4, retryDelay(3)) // ~12s
	assertInRange(t, retryBaseDelay*8, retryDelay(4)) // ~24s
	assertInRange(t, retryMaxDelay, retryDelay(5))    // ~48s
	assertInRange(t, retryMaxDelay, retryDelay(10))   // clamped at ~48s
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
		seen[retryDelay(1)] = struct{}{}
	}
	require.Greater(t, len(seen), 1,
		"retryDelay should produce varied values due to jitter")
}
