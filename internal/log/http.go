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
	if !slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		return http.DefaultClient
	}
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
	start := time.Now()
	resp, err := h.Transport.RoundTrip(req)
	duration := time.Since(start)
	if err != nil {
		slog.Error(
			"HTTP request failed",
			"method", req.Method,
			"url", req.URL,
			"duration_ms", duration.Milliseconds(),
			"errorr", err,
		)
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

	if len(body) > 10000 {
		body = body[:10000]
		body = append(body, []byte("... (truncated)")...)
	}

	slog.Debug(
		"HTTP Response",
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"headers", formatHeaders(resp.Header),
		"body", string(body),
		"content_length", len(body),
		"duration_ms", duration.Milliseconds(),
	)
	return resp, nil
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
