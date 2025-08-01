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

// NewHTTPClient creates an HTTP client with debug logging enabled when debug mode is on.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: &HTTPRoundTripLogger{
			Transport: http.DefaultTransport,
		},
	}
}

// HTTPRoundTripLogger is an http.RoundTripper that logs requests and responses.
type HTTPRoundTripLogger struct {
	Transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper interface with logging.
func (h *HTTPRoundTripLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	if !slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		return h.Transport.RoundTrip(req)
	}

	start := time.Now()
	resp, err := h.Transport.RoundTrip(req)
	duration := time.Since(start)
	if err != nil {
		logHTTPError(req.Method, req.URL.String(), 0, nil, nil, duration)
		return resp, err
	}

	var body []byte
	if resp.Body != nil {
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			slog.Debug("Failed to read response body for logging", "error", err)
		} else {
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	if resp.StatusCode >= 400 {
		logHTTPError(req.Method, req.URL.String(), resp.StatusCode, body, resp.Header, duration)
	} else {
		logHTTPResponse(resp, body, duration)
	}

	return resp, nil
}

func logHTTPResponse(resp *http.Response, body []byte, duration time.Duration) {
	if !slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		return
	}

	if len(body) > 10000 {
		body = body[:10000]
		body = append(body, []byte("... (truncated)")...)
	}

	// Log response details
	slog.Debug(
		"HTTP Response",
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"headers", formatHeaders(resp.Header),
		"body", string(body),
		"content_length", len(body),
		"duration_ms", duration.Milliseconds(),
	)
}

// logHTTPError logs detailed HTTP error information.
func logHTTPError(method, url string, statusCode int, body []byte, headers http.Header, duration time.Duration) {
	if len(body) > 10000 {
		body = body[:10000]
		body = append(body, []byte("... (truncated)")...)
	}
	slog.Error(
		"HTTP Request Failed",
		"method", method,
		"url", url,
		"status_code", statusCode,
		"headers", formatHeaders(headers),
		"body", string(body),
		"duration_ms", duration.Milliseconds(),
	)
}

// formatHeaders formats HTTP headers for logging, filtering out sensitive information.
func formatHeaders(headers http.Header) map[string][]string {
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
