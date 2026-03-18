package agent

import (
	"errors"
	"strings"
	"time"

	"charm.land/fantasy"
)

const (
	// maxRetriableAttempts is the maximum number of retry attempts for
	// transient errors (429 rate limit, 503 service unavailable).
	maxRetriableAttempts = 5

	// retryBaseDelay is the base delay for exponential backoff.
	retryBaseDelay = 2 * time.Second

	// retryMaxDelay is the maximum delay between retries.
	retryMaxDelay = 60 * time.Second
)

// isRetriableError reports whether the error is a transient error that
// should be retried with exponential backoff. This includes:
//   - 429 Too Many Requests (rate limiting)
//   - 503 Service Unavailable (temporary overload)
//   - Network-level errors that may be transient
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	var providerErr *fantasy.ProviderError
	if !errors.As(err, &providerErr) {
		// Non-provider errors (e.g., network timeouts) are retriable.
		return isTransientNetworkError(err)
	}

	// 429: Rate limit exceeded - always retriable.
	if providerErr.StatusCode == 429 {
		return true
	}

	// 503: Service unavailable - retriable.
	if providerErr.StatusCode == 503 {
		return true
	}

	// 502/504: Gateway errors - often transient.
	if providerErr.StatusCode == 502 || providerErr.StatusCode == 504 {
		return true
	}

	// Check message for rate-limit indicators even with other status codes.
	msg := strings.ToLower(providerErr.Message)
	if strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "overloaded") ||
		strings.Contains(msg, "temporarily unavailable") {
		return true
	}

	return false
}

// isTransientNetworkError reports whether a non-ProviderError is a transient
// network issue that should be retried.
func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common transient network errors.
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "eof") ||
		strings.Contains(errStr, "broken pipe")
}

// retryDelay calculates the delay for the given attempt number using
// exponential backoff.
func retryDelay(attempt int) time.Duration {
	// Exponential backoff: base * 2^attempt
	delay := retryBaseDelay
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay > retryMaxDelay {
			delay = retryMaxDelay
			break
		}
	}
	return delay
}
