package agent

import (
	"log/slog"
	"sync"
	"time"

	"charm.land/fantasy"
)

// CircuitBreaker states
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern for retry handling.
// It monitors error rates and trips the circuit open when thresholds are exceeded.
type CircuitBreaker struct {
	mu sync.Mutex

	// Configuration
	failureThreshold int           // Number of failures before opening circuit
	successThreshold int           // Number of successes in half-open before closing
	timeout          time.Duration // Time to wait before attempting recovery

	// State
	state            CircuitState
	failureCount     int
	successCount     int
	lastFailureTime  time.Time
	lastFailure      *fantasy.ProviderError

	// Metrics
	totalFailures    int
	totalSuccesses   int
	totalTrips       int
	lastTripTime     time.Time
}

// NewCircuitBreaker creates a new CircuitBreaker with the given configuration.
func NewCircuitBreaker(failureThreshold int, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		state:            CircuitClosed,
	}
}

// RecordFailure records a failure and returns the delay to wait before retry.
// If the circuit is open, it returns a delay of -1 to indicate immediate rejection.
func (cb *CircuitBreaker) RecordFailure(err *fantasy.ProviderError) (delay time.Duration, shouldTrip bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = err
	cb.lastFailureTime = time.Now()
	cb.failureCount++
	cb.totalFailures++

	switch cb.state {
	case CircuitClosed:
		if cb.failureCount >= cb.failureThreshold {
			cb.trip()
			return -1, true
		}
		// Exponential backoff: base * 2^failures, max 30 seconds
		delay := time.Duration(1<<uint(min(cb.failureCount, 5))) * time.Second
		return delay, false

	case CircuitOpen:
		// Still open, reject immediately
		return -1, false

	case CircuitHalfOpen:
		// One more failure trips it open again
		cb.trip()
		return -1, true

	default:
		return time.Second, false
	}
}

// RecordSuccess records a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++
	cb.totalSuccesses++
	cb.failureCount = 0 // Reset failure count on success

	if cb.state == CircuitHalfOpen {
		if cb.successCount >= cb.successThreshold {
			cb.reset()
		}
	}
}

// trip opens the circuit breaker.
func (cb *CircuitBreaker) trip() {
	cb.state = CircuitOpen
	cb.totalTrips++
	cb.lastTripTime = time.Now()
	cb.successCount = 0
}

// reset closes the circuit breaker.
func (cb *CircuitBreaker) reset() {
	cb.state = CircuitClosed
	cb.failureCount = 0
	cb.successCount = 0
}

// Allow checks if a request should be allowed through.
// Returns true if the circuit is closed or half-open.
// Returns false if the circuit is open and the timeout hasn't passed.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitClosed {
		return true
	}

	if cb.state == CircuitOpen {
		// Check if timeout has passed, transition to half-open
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = CircuitHalfOpen
			cb.successCount = 0
			return true
		}
		return false
	}

	// Half-open state allows requests through for testing
	return true
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// LastFailure returns the most recent failure error.
func (cb *CircuitBreaker) LastFailure() *fantasy.ProviderError {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.lastFailure
}

// Metrics returns current circuit breaker metrics.
func (cb *CircuitBreaker) Metrics() CircuitBreakerMetrics {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return CircuitBreakerMetrics{
		State:           cb.state.String(),
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		TotalFailures:   cb.totalFailures,
		TotalSuccesses:  cb.totalSuccesses,
		TotalTrips:      cb.totalTrips,
		LastFailureTime: cb.lastFailureTime,
		LastTripTime:    cb.lastTripTime,
	}
}

// CircuitBreakerMetrics holds metrics for monitoring.
type CircuitBreakerMetrics struct {
	State           string
	FailureCount    int
	SuccessCount    int
	TotalFailures   int
	TotalSuccesses  int
	TotalTrips      int
	LastFailureTime time.Time
	LastTripTime    time.Time
}

// Global circuit breaker for the agent (can be customized per-session if needed)
var globalCircuitBreaker = NewCircuitBreaker(5, 2, 30*time.Second)

// GlobalCircuitBreaker returns the global circuit breaker instance.
func GlobalCircuitBreaker() *CircuitBreaker {
	return globalCircuitBreaker
}

// OnRetryHandler is the callback function for handling retries with circuit breaker support.
func OnRetryHandler(err *fantasy.ProviderError, delay time.Duration) {
	cb := GlobalCircuitBreaker()

	// Determine appropriate delay based on error type
	var retryDelay time.Duration
	var shouldTrip bool

	// Check if this is a rate limit error (429) or server error (5xx)
	if err != nil && err.StatusCode != 0 {
		switch err.StatusCode {
		case 429: // Too Many Requests
			// Rate limiting - use longer backoff
			retryDelay = delay * 2
			// Record the failure but don't trip for rate limits
			cb.RecordFailure(err)
			return
		case 500, 502, 503, 504: // Server errors
			retryDelay, shouldTrip = cb.RecordFailure(err)
		default:
			retryDelay, shouldTrip = cb.RecordFailure(err)
		}
	} else {
		// Network or other errors
		retryDelay, shouldTrip = cb.RecordFailure(err)
	}

	if shouldTrip {
		// Circuit just tripped - log the event
		metrics := cb.Metrics()
		errMsg := ""
		if err != nil {
			errMsg = err.Message
		}
		slog.Warn("Circuit breaker tripped",
			"total_failures", metrics.TotalFailures,
			"last_error", errMsg,
		)
	}

	// If delay is -1, the circuit is open and we should not retry
	if retryDelay == -1 {
		return
	}

	// If the suggested delay is less than what we calculated, use the larger value
	if delay > 0 && delay > retryDelay {
		retryDelay = delay
	}
}
