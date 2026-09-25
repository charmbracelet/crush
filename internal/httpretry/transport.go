// Package httpretry provides an [http.RoundTripper] that retries requests
// which failed for transient, transport-level reasons: a connection that
// was reset or refused, a server that closed the connection before
// answering, a temporary DNS failure, a 408/502/503/504 response, or a 429
// that says when to retry.
//
// Only requests that are safe to replay are retried: idempotent methods
// (GET, HEAD, OPTIONS, TRACE), or any method whose context was marked with
// [MarkIdempotent], and only when the body can be rewound. Everything else
// passes straight through to the base transport.
//
// The retry is scoped to the single HTTP request. It never involves the
// caller, so a tool that fetches a URL sees one call and one result, and
// a caller that retries at a higher level (an agent loop re-asking a
// model, for example) is never needed for a network blip.
package httpretry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultMaxAttempts is the total number of attempts, including the
	// first one.
	DefaultMaxAttempts = 3
	// DefaultBaseDelay is the delay before the first retry; it doubles on
	// every subsequent retry.
	DefaultBaseDelay = 250 * time.Millisecond
	// DefaultMaxDelay caps the exponential backoff.
	DefaultMaxDelay = 2 * time.Second

	// maxRetryAfter caps a Retry-After header. A server asking for longer
	// than this gets its response back unretried: the wait does not fit a
	// tool call, and retrying sooner would ignore what the server asked.
	maxRetryAfter = 10 * time.Second
	// minAttemptTime is how much time must be left before the deadline,
	// after the backoff, for a retry to be worth starting.
	minAttemptTime = time.Second
	// drainLimit bounds how much of a discarded response body is read so
	// the connection can be reused.
	drainLimit = 64 << 10
)

// Transport retries transient failures of replayable requests.
type Transport struct {
	// Base performs the actual requests. Nil means [http.DefaultTransport].
	Base http.RoundTripper
	// MaxAttempts is the total number of attempts. Zero means
	// [DefaultMaxAttempts].
	MaxAttempts int
	// BaseDelay is the delay before the first retry. Zero means
	// [DefaultBaseDelay].
	BaseDelay time.Duration
	// MaxDelay caps the backoff. Zero means [DefaultMaxDelay].
	MaxDelay time.Duration

	// sleep waits for the backoff delay. Tests replace it.
	sleep func(ctx context.Context, d time.Duration) error
}

// New returns a Transport with the default policy on top of base.
func New(base http.RoundTripper) *Transport {
	return &Transport{Base: base}
}

type idempotentKey struct{}

// MarkIdempotent returns a context under which a request with a
// non-idempotent method (POST, typically a read-only query) may be
// retried. Only mark requests that are safe to send twice.
func MarkIdempotent(ctx context.Context) context.Context {
	return context.WithValue(ctx, idempotentKey{}, true)
}

func markedIdempotent(ctx context.Context) bool {
	v, _ := ctx.Value(idempotentKey{}).(bool)
	return v
}

// RoundTrip implements [http.RoundTripper].
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if !Replayable(req) {
		return base.RoundTrip(req)
	}

	maxAttempts := t.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}
	ctx := req.Context()

	for attempt := 1; ; attempt++ {
		r := req
		if attempt > 1 {
			r = req.Clone(ctx)
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				r.Body = body
			}
		}

		resp, err := base.RoundTrip(r)

		var reason string
		var delay time.Duration
		switch {
		case err != nil:
			if attempt >= maxAttempts || !RetryableError(err) {
				return nil, err
			}
			reason = err.Error()
			delay = t.backoff(attempt)
		case RetryableStatus(resp.StatusCode):
			if attempt >= maxAttempts {
				return resp, nil
			}
			ra, hasRetryAfter := retryAfter(resp.Header)
			if ra > maxRetryAfter {
				return resp, nil
			}
			// A rate limit rarely lifts within the backoff, so a 429 is
			// retried only when the server says when to come back.
			if !hasRetryAfter && resp.StatusCode == http.StatusTooManyRequests {
				return resp, nil
			}
			reason = resp.Status
			delay = t.backoff(attempt)
			if ra > 0 {
				delay = ra
			}
		default:
			return resp, nil
		}

		// A retry without time to finish before the deadline would most
		// likely fail with "context deadline exceeded" and hide why the
		// request failed. Report this attempt's outcome instead.
		if !fitsBeforeDeadline(ctx, delay) {
			return resp, err
		}
		if resp != nil {
			drain(resp.Body)
		}

		slog.Debug("Retrying HTTP request",
			"method", req.Method,
			"url", redact(req),
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"delay", delay,
			"reason", reason,
		)
		if err := t.wait(ctx, delay); err != nil {
			return nil, err
		}
	}
}

// Replayable reports whether req may be sent more than once: its method
// is idempotent (or its context was marked with [MarkIdempotent]) and its
// body, if any, can be rewound.
func Replayable(req *http.Request) bool {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
	default:
		if !markedIdempotent(req.Context()) {
			return false
		}
	}
	return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
}

// transportErrorFragments match transport failures that only surface as
// text: Go's bundled HTTP/2 error types and net/http's idle-connection
// error are unexported. Socket errors are matched by errno instead (see
// connErrnos), since their text is localized on some platforms.
var transportErrorFragments = []string{
	"http2: server sent GOAWAY",
	"stream error:",
	"connection error:",
	"server closed idle connection",
}

// RetryableError reports whether err is a transient transport failure. It
// deliberately does not use net.Error: any syscall.Errno satisfies that
// interface, so a filesystem error would qualify too.
func RetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsTemporary || dnsErr.IsTimeout
	}
	// The server accepted the connection and closed it before answering.
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Timeout() {
			return true
		}
		for _, errno := range connErrnos {
			if errors.Is(err, errno) {
				return true
			}
		}
		return false
	}
	msg := err.Error()
	for _, fragment := range transportErrorFragments {
		if strings.Contains(msg, fragment) {
			return true
		}
	}
	return false
}

// RetryableStatus reports whether a response status is worth retrying.
// 500 is left out: from an arbitrary site it usually repeats, and the
// caller still sees the response. RoundTrip retries a 429 only when it
// carries a Retry-After header.
func RetryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// retryAfter parses a Retry-After header, in whole seconds or as an HTTP
// date (RFC 9110, section 10.2.3), and reports whether it is present and
// well formed. A date in the past yields a duration of zero or less.
func retryAfter(h http.Header) (time.Duration, bool) {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0, false
	}
	// A value too large for 32 bits is still a valid, very long wait:
	// ParseUint reports ErrRange and returns the largest value, which
	// converts to a Duration without overflow.
	if secs, err := strconv.ParseUint(v, 10, 32); err == nil || errors.Is(err, strconv.ErrRange) {
		return time.Duration(secs) * time.Second, true
	}
	if at, err := http.ParseTime(v); err == nil {
		return time.Until(at), true
	}
	return 0, false
}

func (t *Transport) backoff(attempt int) time.Duration {
	base := t.BaseDelay
	if base <= 0 {
		base = DefaultBaseDelay
	}
	maxDelay := t.MaxDelay
	if maxDelay <= 0 {
		maxDelay = DefaultMaxDelay
	}
	d := base << (attempt - 1)
	if d > maxDelay || d <= 0 {
		d = maxDelay
	}
	// ±25% jitter so simultaneous callers do not retry in lockstep.
	jitter := time.Duration((rand.Float64() - 0.5) * 0.5 * float64(d))
	return d + jitter
}

// fitsBeforeDeadline reports whether, after waiting d, at least
// minAttemptTime would remain before ctx's deadline for another attempt.
// http.Client.Timeout sets that deadline, so it bounds the retries as well.
func fitsBeforeDeadline(ctx context.Context, d time.Duration) bool {
	deadline, ok := ctx.Deadline()
	return !ok || time.Until(deadline) > d+minAttemptTime
}

func (t *Transport) wait(ctx context.Context, d time.Duration) error {
	if t.sleep != nil {
		return t.sleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func drain(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, drainLimit))
	_ = body.Close()
}

// redact keeps scheme, host and path for logging; query strings and
// userinfo can carry credentials.
func redact(req *http.Request) string {
	if req.URL == nil {
		return ""
	}
	u := *req.URL
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	return u.String()
}
