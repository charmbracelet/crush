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
	// Retry schedule: 3s, 6s, 12s, 24s, 48s.
	retryBaseDelay = 3 * time.Second

	// retryMaxDelay is the maximum delay between retries.
	retryMaxDelay = 48 * time.Second
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
	if errors.As(err, &providerErr) {
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
	}

	// Fallback: check the full error string for HTTP status codes that
	// indicate transient failures. This catches errors that are not
	// wrapped as *fantasy.ProviderError (e.g. *fantasy.Error or raw
	// HTTP client errors).
	// Use more specific patterns to avoid false positives.
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "status 503") || strings.Contains(errStr, "http 503") ||
		strings.Contains(errStr, "service unavailable") ||
		strings.Contains(errStr, "status 502") || strings.Contains(errStr, "http 502") ||
		strings.Contains(errStr, "bad gateway") ||
		strings.Contains(errStr, "status 504") || strings.Contains(errStr, "http 504") ||
		strings.Contains(errStr, "gateway timeout") ||
		strings.Contains(errStr, "status 429") || strings.Contains(errStr, "http 429") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "overloaded") {
		return true
	}

	return isTransientNetworkError(err)
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
// exponential backoff: base * 2^(attempt-1).
// With base=3s: 3s, 6s, 12s, 24s, 48s.
func retryDelay(attempt int) time.Duration {
	delay := retryBaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay > retryMaxDelay {
			delay = retryMaxDelay
			break
		}
	}
	return delay
}
