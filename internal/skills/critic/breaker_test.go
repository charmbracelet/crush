package critic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBreakerRegistry_CanExecute_AllowsWhenClosed(t *testing.T) {
	t.Parallel()
	br := newBreakerRegistry()
	require.NoError(t, br.CanExecute("s1"))
}

func TestBreakerRegistry_CanExecute_BlocksWhenOpen(t *testing.T) {
	t.Parallel()
	br := newBreakerRegistry()
	retryable := context.DeadlineExceeded

	for i := 0; i < breakerFailureThreshold; i++ {
		require.ErrorIs(t, br.RecordResult("s1", retryable), retryable)
	}
	require.ErrorIs(t, br.CanExecute("s1"), ErrCircuitOpen)
}

func TestBreakerRegistry_CanExecute_AllowsAfterCooldown(t *testing.T) {
	t.Parallel()
	br := newBreakerRegistry()
	retryable := context.DeadlineExceeded

	for i := 0; i < breakerFailureThreshold; i++ {
		_ = br.RecordResult("s1", retryable)
	}
	require.ErrorIs(t, br.CanExecute("s1"), ErrCircuitOpen)

	br.mu.Lock()
	br.breakers["s1"].lastFailureAt = time.Now().Add(-breakerCooldown - time.Second)
	br.mu.Unlock()

	require.NoError(t, br.CanExecute("s1"))
}

func TestBreakerRegistry_SuccessCloses(t *testing.T) {
	t.Parallel()
	br := newBreakerRegistry()
	require.NoError(t, br.RecordResult("s1", nil))
}

func TestBreakerRegistry_OpensAfterFailures(t *testing.T) {
	t.Parallel()
	br := newBreakerRegistry()
	retryable := context.DeadlineExceeded

	for i := 0; i < breakerFailureThreshold; i++ {
		require.ErrorIs(t, br.RecordResult("s1", retryable), retryable)
	}
	// Circuit is now open. Next call returns ErrCircuitOpen.
	require.ErrorIs(t, br.RecordResult("s1", retryable), ErrCircuitOpen)
	require.ErrorIs(t, br.RecordResult("s1", retryable), ErrCircuitOpen)
}

func TestBreakerRegistry_NonRetryableDoesNotOpen(t *testing.T) {
	t.Parallel()
	br := newBreakerRegistry()
	nonRetryable := errors.New("auth failed")

	for i := 0; i < breakerFailureThreshold*2; i++ {
		require.ErrorIs(t, br.RecordResult("s1", nonRetryable), nonRetryable)
	}
	// Circuit should still be closed because errors are non-retryable.
	require.ErrorIs(t, br.RecordResult("s1", nonRetryable), nonRetryable)
}

func TestBreakerRegistry_HalfOpenAfterCooldown(t *testing.T) {
	t.Parallel()
	br := newBreakerRegistry()
	retryable := context.DeadlineExceeded

	// Open the circuit.
	for i := 0; i < breakerFailureThreshold; i++ {
		_ = br.RecordResult("s1", retryable)
	}
	require.ErrorIs(t, br.RecordResult("s1", retryable), ErrCircuitOpen)

	// Manually age the breaker.
	br.mu.Lock()
	br.breakers["s1"].lastFailureAt = time.Now().Add(-breakerCooldown - time.Second)
	br.mu.Unlock()

	// Next call goes through (half-open) but still fails.
	require.ErrorIs(t, br.RecordResult("s1", retryable), retryable)
}

func TestBreakerRegistry_SuccessResets(t *testing.T) {
	t.Parallel()
	br := newBreakerRegistry()
	retryable := context.DeadlineExceeded

	// Open the circuit.
	for i := 0; i < breakerFailureThreshold; i++ {
		_ = br.RecordResult("s1", retryable)
	}
	require.ErrorIs(t, br.RecordResult("s1", retryable), ErrCircuitOpen)

	// Manually age the breaker.
	br.mu.Lock()
	br.breakers["s1"].lastFailureAt = time.Now().Add(-breakerCooldown - time.Second)
	br.mu.Unlock()

	// Success resets to closed.
	require.NoError(t, br.RecordResult("s1", nil))
	require.NoError(t, br.RecordResult("s1", nil))
}

func TestBreakerRegistry_Cleanup(t *testing.T) {
	t.Parallel()
	br := newBreakerRegistry()
	retryable := context.DeadlineExceeded

	_ = br.RecordResult("old", retryable)

	// Age the breaker.
	br.mu.Lock()
	br.breakers["old"].lastAccessAt = time.Now().Add(-breakerCleanupInterval - time.Second)
	br.mu.Unlock()

	// A new record triggers cleanup.
	_ = br.RecordResult("new", nil)

	br.mu.Lock()
	_, exists := br.breakers["old"]
	br.mu.Unlock()
	require.False(t, exists, "old breaker should be cleaned up")
}

func TestIsRetryableError(t *testing.T) {
	t.Parallel()
	require.True(t, isRetryableError(context.DeadlineExceeded))
	require.False(t, isRetryableError(errors.New("auth failed")))
	require.False(t, isRetryableError(nil))
}
