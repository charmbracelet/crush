package log

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/sethvargo/go-retry"
)

// DefaultRetryConfig provides sensible defaults for retrying HTTP requests.
var DefaultRetryConfig = RetryConfig{
	MaxRetries:  3,
	BaseBackoff: 500 * time.Millisecond,
	MaxBackoff:  30 * time.Second,
	Jitter:      250 * time.Millisecond,
}

// RetryConfig configures retry behavior for HTTP requests.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (not including the
	// initial request).
	MaxRetries uint64
	// BaseBackoff is the initial backoff duration before exponential increase.
	BaseBackoff time.Duration
	// MaxBackoff caps the maximum backoff duration between retries.
	MaxBackoff time.Duration
	// Jitter adds randomness to backoff to prevent thundering herd.
	Jitter time.Duration
}

// RetryTransport is an http.RoundTripper that retries transient failures with
// exponential backoff.
type RetryTransport struct {
	Transport http.RoundTripper
	Config    RetryConfig
}

// NewRetryTransport creates a new RetryTransport with the given config.
func NewRetryTransport(transport http.RoundTripper, cfg RetryConfig) *RetryTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &RetryTransport{
		Transport: transport,
		Config:    cfg,
	}
}

// RoundTrip implements http.RoundTripper with retry logic.
func (rt *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Build backoff strategy: exponential with jitter, capped, and limited retries.
	backoff := retry.NewExponential(rt.Config.BaseBackoff)
	backoff = retry.WithCappedDuration(rt.Config.MaxBackoff, backoff)
	backoff = retry.WithJitter(rt.Config.Jitter, backoff)
	backoff = retry.WithMaxRetries(rt.Config.MaxRetries, backoff)

	// Buffer the request body so we can retry.
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
	}

	var resp *http.Response
	var lastResp *http.Response
	var attempt int

	err := retry.Do(req.Context(), backoff, func(ctx context.Context) error {
		attempt++

		// Reset body for each attempt.
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		var err error
		resp, err = rt.Transport.RoundTrip(req)
		if err != nil {
			if rt.isRetryableError(err) {
				slog.Warn("HTTP request failed, retrying",
					"method", req.Method,
					"url", req.URL.String(),
					"attempt", attempt,
					"error", err,
				)
				return retry.RetryableError(err)
			}
			// Non-retryable error.
			return err
		}

		// Check for retryable HTTP status codes.
		if rt.isRetryableStatus(resp.StatusCode) {
			// Keep the last response in case retries are exhausted.
			lastResp = resp

			delay := rt.getRetryAfter(resp)
			slog.Warn("HTTP request returned retryable status, retrying",
				"method", req.Method,
				"url", req.URL.String(),
				"status", resp.StatusCode,
				"attempt", attempt,
				"retry_after", delay,
			)
			return retry.RetryableError(errors.New(resp.Status))
		}

		return nil
	})
	// If retries exhausted but we have a response, return it (caller can check
	// status code).
	if err != nil {
		if lastResp != nil {
			return lastResp, nil
		}
		return nil, err
	}

	return resp, nil
}

// isRetryableError checks if the error is a transient network error that
// should be retried.
func (rt *RetryTransport) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors (timeout, connection refused, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary() //nolint:staticcheck // Temporary() is deprecated but still useful
	}

	// Check for connection reset, EOF, and similar I/O errors.
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// Check for specific error strings that indicate transient failures.
	errStr := err.Error()
	transientPatterns := []string{
		"connection reset",
		"connection refused",
		"no such host",
		"network is unreachable",
		"i/o timeout",
		"TLS handshake timeout",
		"context deadline exceeded",
	}
	for _, pattern := range transientPatterns {
		if bytes.Contains([]byte(errStr), []byte(pattern)) {
			return true
		}
	}

	return false
}

// isRetryableStatus checks if the HTTP status code indicates a retryable
// condition.
func (rt *RetryTransport) isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	default:
		return false
	}
}

// getRetryAfter parses the Retry-After header if present.
func (rt *RetryTransport) getRetryAfter(resp *http.Response) time.Duration {
	header := resp.Header.Get("Retry-After")
	if header == "" {
		return 0
	}

	// Try parsing as seconds.
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP date.
	if t, err := http.ParseTime(header); err == nil {
		return time.Until(t)
	}

	return 0
}

// NewHTTPClientWithRetry creates an HTTP client with retry and optional debug
// logging.
func NewHTTPClientWithRetry(debug bool) *http.Client {
	var transport http.RoundTripper = http.DefaultTransport

	// Add retry layer.
	transport = NewRetryTransport(transport, DefaultRetryConfig)

	// Add logging layer on top if debug is enabled.
	if debug {
		transport = &HTTPRoundTripLogger{Transport: transport}
	}

	return &http.Client{
		Transport: transport,
	}
}
