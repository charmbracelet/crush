package critic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = fmt.Errorf("critic circuit breaker is open")

const (
	breakerFailureThreshold = 5
	breakerCooldown         = 30 * time.Second
	breakerCleanupInterval  = 10 * time.Minute
)

// breakerState represents the state of a circuit breaker.
type breakerState int

const (
	breakerClosed breakerState = iota
	breakerOpen
	breakerHalfOpen
)

// circuitBreaker is a per-session failure detector.
type circuitBreaker struct {
	state         breakerState
	failures      int
	lastFailureAt time.Time
	lastAccessAt  time.Time
}

// breakerRegistry holds circuit breakers keyed by session ID.
type breakerRegistry struct {
	mu       sync.Mutex
	breakers map[string]*circuitBreaker
}

func newBreakerRegistry() *breakerRegistry {
	return &breakerRegistry{
		breakers: make(map[string]*circuitBreaker),
	}
}

// RecordResult updates the breaker for the given session based on whether the
// call succeeded. It returns an error if the breaker is open.
func (br *breakerRegistry) RecordResult(sessionID string, err error) error {
	br.mu.Lock()
	defer br.mu.Unlock()

	br.cleanupLocked()

	b, ok := br.breakers[sessionID]
	if !ok {
		b = &circuitBreaker{state: breakerClosed}
		br.breakers[sessionID] = b
	}
	b.lastAccessAt = time.Now()

	if err == nil {
		b.state = breakerClosed
		b.failures = 0
		return nil
	}

	if !isRetryableError(err) {
		// Non-retryable errors do not count toward the breaker.
		return err
	}

	switch b.state {
	case breakerClosed:
		b.failures++
		if b.failures >= breakerFailureThreshold {
			b.state = breakerOpen
			b.lastFailureAt = time.Now()
		}
		return err
	case breakerOpen:
		if time.Since(b.lastFailureAt) > breakerCooldown {
			b.state = breakerHalfOpen
			b.failures = 1
			return err
		}
		return ErrCircuitOpen
	case breakerHalfOpen:
		b.failures++
		if b.failures >= breakerFailureThreshold {
			b.state = breakerOpen
			b.lastFailureAt = time.Now()
		} else {
			b.state = breakerClosed
		}
		return err
	}

	return err
}

// cleanupLocked removes breakers that have been inactive for longer than
// breakerCleanupInterval. Must be called with mu held.
func (br *breakerRegistry) cleanupLocked() {
	cutoff := time.Now().Add(-breakerCleanupInterval)
	for id, b := range br.breakers {
		if b.lastAccessAt.Before(cutoff) {
			delete(br.breakers, id)
		}
	}
}

// isRetryableError reports whether an error is likely transient and worth
// retrying. Non-retryable errors (auth, parse, config) should not open the
// circuit breaker.
func isRetryableError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Network timeout errors.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}
