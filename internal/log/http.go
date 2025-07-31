package log

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func logHTTPResponse(resp *http.Response, body []byte, duration time.Duration, err error) {
	if !slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		return
	}

	var bodyStr string
	if len(body) > 0 {
		// Truncate very large bodies for readability
		if len(body) > 10000 {
			bodyStr = string(body[:10000]) + "... (truncated)"
		} else {
			bodyStr = string(body)
		}
	}

	if err != nil {
		slog.Debug("HTTP Response Error",
			"error", err.Error(),
			"duration_ms", duration.Milliseconds(),
		)
		return
	}

	// Log response details
	slog.Debug("HTTP Response",
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"headers", FormatHeaders(resp.Header),
		"body", bodyStr,
		"content_length", len(body),
		"duration_ms", duration.Milliseconds(),
	)
}

// LogHTTPError logs detailed HTTP error information.
func LogHTTPError(method, url string, statusCode int, responseBody []byte, headers http.Header, duration time.Duration) {
	var bodyStr string
	if len(responseBody) > 0 {
		// For error responses, include more of the body since it's likely important
		if len(responseBody) > 50000 {
			bodyStr = string(responseBody[:50000]) + "... (truncated)"
		} else {
			bodyStr = string(responseBody)
		}
	}

	slog.Error("HTTP Request Failed",
		"method", method,
		"url", url,
		"status_code", statusCode,
		"response_headers", FormatHeaders(headers),
		"response_body", bodyStr,
		"duration_ms", duration.Milliseconds(),
	)
}

// FormatHeaders formats HTTP headers for logging, filtering out sensitive information.
func FormatHeaders(headers http.Header) map[string][]string {
	filtered := make(map[string][]string)
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		// Filter out sensitive headers
		if strings.Contains(lowerKey, "authorization") ||
			strings.Contains(lowerKey, "api-key") ||
			strings.Contains(lowerKey, "token") ||
			strings.Contains(lowerKey, "secret") {
			filtered[key] = []string{"[REDACTED]"}
		} else {
			filtered[key] = values
		}
	}
	return filtered
}

// HTTPRoundTripLogger is an http.RoundTripper that logs requests and responses.
type HTTPRoundTripLogger struct {
	Transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper interface with logging.
func (h *HTTPRoundTripLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	if !slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		// If debug logging is disabled, just pass through
		return h.Transport.RoundTrip(req)
	}

	start := time.Now()

	// Read and log request body if present
	var reqBody []byte
	if req.Body != nil {
		var err error
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			slog.Debug("Failed to read request body for logging", "error", err)
		} else {
			// Restore the body for the actual request
			req.Body = io.NopCloser(bytes.NewReader(reqBody))
		}
	}

	// Make the actual request
	resp, err := h.Transport.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		logHTTPResponse(nil, nil, duration, err)
		return resp, err
	}

	// Read and log response body
	var respBody []byte
	if resp.Body != nil {
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			slog.Debug("Failed to read response body for logging", "error", err)
		} else {
			// Restore the body for the caller
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
		}
	}

	// Log successful response or error response
	if resp.StatusCode >= 400 {
		LogHTTPError(req.Method, req.URL.String(), resp.StatusCode, respBody, resp.Header, duration)
	} else {
		logHTTPResponse(resp, respBody, duration, nil)
	}

	return resp, nil
}

// NewHTTPClient creates an HTTP client with debug logging enabled when debug mode is on.
func NewHTTPClient(baseTransport http.RoundTripper) *http.Client {
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	return &http.Client{
		Transport: &HTTPRoundTripLogger{
			Transport: baseTransport,
		},
		Timeout: 30 * time.Second,
	}
}
