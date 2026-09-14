package log

import (
	"bytes"
	"encoding/json"
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
	var err error
	var save io.ReadCloser
	save, req.Body, err = drainBody(req.Body)
	if err != nil {
		slog.Error(
			"HTTP request failed",
			"method", req.Method,
			"url", req.URL,
			"error", err,
		)
		return nil, err
	}

	if slog.Default().Enabled(req.Context(), slog.LevelDebug) {
		slog.Debug(
			"HTTP Request",
			"method", req.Method,
			"url", req.URL,
			"body", bodyToString(save),
		)
	}

	start := time.Now()
	resp, err := h.Transport.RoundTrip(req)
	duration := time.Since(start)
	if err != nil {
		slog.Error(
			"HTTP request failed",
			"method", req.Method,
			"url", req.URL,
			"duration_ms", duration.Milliseconds(),
			"error", err,
		)
		return resp, err
	}

	if !slog.Default().Enabled(req.Context(), slog.LevelDebug) {
		return resp, nil
	}

	// Recorded as the body is read, not drained up front: draining waits for
	// EOF, which on a stream only arrives when the turn ends.
	logResponse := func(body string, truncated bool) {
		slog.Debug(
			"HTTP Response",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"headers", formatHeaders(resp.Header),
			"body", body,
			"body_truncated", truncated,
			"content_length", resp.ContentLength,
			"duration_ms", duration.Milliseconds(),
		)
	}
	if resp.Body == nil || resp.Body == http.NoBody {
		logResponse("", false)
		return resp, nil
	}
	resp.Body = &recordingBody{inner: resp.Body, log: logResponse}
	return resp, nil
}

// maxRecordedBodyBytes caps how much of a logged body is held in memory. The
// useful part of a stall is the last events before it, so the tail is kept.
const maxRecordedBodyBytes = 1 << 20

// recordingBody keeps the tail of what is read through a response body and
// logs it once the body ends. Reads pass straight through, so streams stream.
type recordingBody struct {
	inner     io.ReadCloser
	buf       bytes.Buffer
	truncated bool
	logged    bool
	log       func(body string, truncated bool)
}

func (r *recordingBody) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		r.record(p[:n])
	}
	if err != nil {
		r.flush()
	}
	return n, err
}

func (r *recordingBody) Close() error {
	r.flush()
	return r.inner.Close()
}

// record appends to the tail buffer, dropping the oldest bytes once the cap
// is reached.
func (r *recordingBody) record(p []byte) {
	r.buf.Write(p)
	if r.buf.Len() <= maxRecordedBodyBytes {
		return
	}
	r.truncated = true
	tail := r.buf.Bytes()[r.buf.Len()-maxRecordedBodyBytes:]
	kept := make([]byte, maxRecordedBodyBytes)
	copy(kept, tail)
	r.buf.Reset()
	r.buf.Write(kept)
}

// flush writes the log line once, whether the body ended on its own or was
// closed early.
func (r *recordingBody) flush() {
	if r.logged {
		return
	}
	r.logged = true
	r.log(prettyJSON(r.buf.Bytes()), r.truncated)
}

func bodyToString(body io.ReadCloser) string {
	if body == nil {
		return ""
	}
	src, err := io.ReadAll(body)
	if err != nil {
		slog.Error("Failed to read body", "error", err)
		return ""
	}
	return prettyJSON(src)
}

// prettyJSON indents src when it is JSON and returns it unchanged otherwise.
func prettyJSON(src []byte) string {
	var b bytes.Buffer
	if json.Indent(&b, bytes.TrimSpace(src), "", "  ") != nil {
		return string(src)
	}
	return b.String()
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

func drainBody(b io.ReadCloser) (r1, r2 io.ReadCloser, err error) {
	if b == nil || b == http.NoBody {
		return http.NoBody, http.NoBody, nil
	}
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, b, err
	}
	if err = b.Close(); err != nil {
		return nil, b, err
	}
	return io.NopCloser(&buf), io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}
