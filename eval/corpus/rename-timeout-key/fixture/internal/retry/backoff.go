// Package retry derives retry budgets from the request timeout.
package retry

import (
	"time"

	"taskapi/internal/config"
)

// Policy describes how retries are budgeted against the request
// timeout so a retry loop can never outlive the caller's deadline.
type Policy struct {
	// Budget is the total wall time available to all attempts.
	Budget time.Duration

	// BaseBackoff is the sleep before the first retry; each retry
	// doubles it.
	BaseBackoff time.Duration

	// MaxAttempts caps the number of tries including the first.
	MaxAttempts int
}

// DefaultPolicy derives a Policy from the service config: the retry
// budget is the configured timeout, spread across at most three
// attempts.
func DefaultPolicy(cfg config.Config) Policy {
	return Policy{
		Budget:      time.Duration(cfg.TimeoutMS) * time.Second,
		BaseBackoff: 100 * time.Millisecond,
		MaxAttempts: 3,
	}
}

// ForTimeout builds a Policy from an explicit timeout rather than the
// config — used by callers that override per-request.
func ForTimeout(timeoutSeconds int) Policy {
	return Policy{
		Budget:      time.Duration(timeoutSeconds) * time.Second,
		BaseBackoff: 100 * time.Millisecond,
		MaxAttempts: 3,
	}
}

// Backoff returns the sleep before attempt n (1-based).
func (p Policy) Backoff(n int) time.Duration {
	if n <= 1 {
		return 0
	}
	d := p.BaseBackoff
	for i := 2; i < n; i++ {
		d *= 2
	}
	return d
}
