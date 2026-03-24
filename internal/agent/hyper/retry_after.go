package hyper

import "time"

// RetryAfterError wraps an error with a Retry-After duration extracted from
// an HTTP response header. The retry loop uses this to respect server-
// requested backoff periods.
type RetryAfterError struct {
	Err   error
	After time.Duration
}

func (e *RetryAfterError) Error() string { return e.Err.Error() }
func (e *RetryAfterError) Unwrap() error { return e.Err }
