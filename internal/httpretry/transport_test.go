package httpretry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// get issues a GET for url on the test's context.
func get(t *testing.T, client *http.Client, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	return client.Do(req)
}

// newClient returns a client whose transport records backoff delays
// instead of sleeping, so tests run instantly and can assert on them.
func newClient(t *testing.T) (*http.Client, *[]time.Duration) {
	t.Helper()
	delays := &[]time.Duration{}
	tr := New(http.DefaultTransport.(*http.Transport).Clone())
	tr.sleep = func(ctx context.Context, d time.Duration) error {
		*delays = append(*delays, d)
		return ctx.Err()
	}
	return &http.Client{Transport: tr, Timeout: 5 * time.Second}, delays
}

// resetOnceServer accepts the first connection and closes it with RST
// before answering, then serves "ok".
func resetOnceServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			require.NoError(t, err)
			if tcp, ok := conn.(*net.TCPConn); ok {
				_ = tcp.SetLinger(0)
			}
			_ = conn.Close()
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func statusServer(t *testing.T, codes ...int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		code := codes[len(codes)-1]
		if n <= len(codes) {
			code = codes[n-1]
		}
		if code != http.StatusOK && r.URL.Query().Get("retry_after") != "" {
			w.Header().Set("Retry-After", r.URL.Query().Get("retry_after"))
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestRoundTrip_RecoversFromConnectionReset(t *testing.T) {
	t.Parallel()
	srv, calls := resetOnceServer(t)
	client, delays := newClient(t)

	resp, err := get(t, client, srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, "ok", string(body))
	assert.EqualValues(t, 2, calls.Load())
	assert.Len(t, *delays, 1)
}

func TestRoundTrip_RetriesStatusAndHonorsRetryAfter(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusServiceUnavailable, http.StatusOK)
	client, delays := newClient(t)

	resp, err := get(t, client, srv.URL+"/?retry_after=1")
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 2, calls.Load())
	assert.Equal(t, []time.Duration{time.Second}, *delays)
}

// A server that asks for a longer pause than a tool call can afford gets
// its answer back as is, not two more requests within a second.
func TestRoundTrip_ReturnsStatusWhenRetryAfterIsTooLong(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusTooManyRequests, http.StatusOK)
	client, delays := newClient(t)

	resp, err := get(t, client, srv.URL+"/?retry_after=60")
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.EqualValues(t, 1, calls.Load())
	assert.Empty(t, *delays)
}

// Retry-After: 0 names no pause, so the usual backoff applies.
func TestRoundTrip_ZeroRetryAfterUsesBackoff(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusTooManyRequests, http.StatusOK)
	client, delays := newClient(t)

	resp, err := get(t, client, srv.URL+"/?retry_after=0")
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 2, calls.Load())
	require.Len(t, *delays, 1)
	assert.Positive(t, (*delays)[0])
}

// Without Retry-After a rate limit is unlikely to lift within the backoff,
// so the 429 goes back to the caller instead of two more requests.
func TestRoundTrip_DoesNotRetryRateLimitWithoutRetryAfter(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusTooManyRequests, http.StatusOK)
	client, delays := newClient(t)

	resp, err := get(t, client, srv.URL)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.EqualValues(t, 1, calls.Load())
	assert.Empty(t, *delays)
}

func TestRoundTrip_RetriesRateLimitWithRetryAfter(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusTooManyRequests, http.StatusOK)
	client, delays := newClient(t)

	resp, err := get(t, client, srv.URL+"/?retry_after=2")
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 2, calls.Load())
	assert.Equal(t, []time.Duration{2 * time.Second}, *delays)
}

func TestRoundTrip_RetriesRequestTimeout(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusRequestTimeout, http.StatusOK)
	client, delays := newClient(t)

	resp, err := get(t, client, srv.URL)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 2, calls.Load())
	assert.Len(t, *delays, 1)
}

func TestRoundTrip_ReturnsLastStatusWhenRetryAfterPassesDeadline(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusServiceUnavailable, http.StatusOK)
	client, delays := newClient(t)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/?retry_after=5", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.EqualValues(t, 1, calls.Load())
	assert.Empty(t, *delays, "must not wait for a retry that cannot start in time")
}

// The wait fits before the deadline, but the attempt after it would have
// almost no time left and fail with a deadline error instead of the 503.
func TestRoundTrip_ReturnsLastStatusWhenNoTimeIsLeftForTheRetry(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusServiceUnavailable, http.StatusOK)
	client, delays := newClient(t)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second+minAttemptTime/2)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/?retry_after=1", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.EqualValues(t, 1, calls.Load())
	assert.Empty(t, *delays)
}

func TestRoundTrip_ReturnsLastErrorWhenBackoffPassesDeadline(t *testing.T) {
	t.Parallel()
	srv, calls := resetOnceServer(t)
	tr := New(http.DefaultTransport.(*http.Transport).Clone())
	tr.BaseDelay = 10 * time.Second
	tr.MaxDelay = 10 * time.Second
	var slept bool
	tr.sleep = func(ctx context.Context, d time.Duration) error {
		slept = true
		return ctx.Err()
	}
	client := &http.Client{Transport: tr}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	require.Error(t, err)

	assert.True(t, RetryableError(err), "the connection failure must be reported, got %v", err)
	assert.False(t, errors.Is(err, context.DeadlineExceeded), "got %v", err)
	assert.EqualValues(t, 1, calls.Load())
	assert.False(t, slept)
}

// http.Client.Timeout puts a deadline on the request context, so the
// same rule holds for the clients tools actually use.
func TestRoundTrip_ClientTimeoutBoundsRetries(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusServiceUnavailable, http.StatusOK)
	client := &http.Client{
		Transport: New(http.DefaultTransport.(*http.Transport).Clone()),
		Timeout:   time.Second,
	}

	start := time.Now()
	resp, err := get(t, client, srv.URL+"/?retry_after=5")
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.EqualValues(t, 1, calls.Load())
	assert.Less(t, time.Since(start), time.Second)
}

func TestRoundTrip_DoesNotRetryPOST(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusServiceUnavailable)
	client, delays := newClient(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, strings.NewReader("payload"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.EqualValues(t, 1, calls.Load())
	assert.Empty(t, *delays)
}

func TestRoundTrip_RetriesPOSTMarkedIdempotent(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusServiceUnavailable, http.StatusOK)
	client, delays := newClient(t)

	req, err := http.NewRequestWithContext(MarkIdempotent(context.Background()), http.MethodPost, srv.URL, strings.NewReader("query"))
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 2, calls.Load())
	assert.Len(t, *delays, 1)
}

func TestRoundTrip_DoesNotRetryClientErrors(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusNotFound)
	client, delays := newClient(t)

	resp, err := get(t, client, srv.URL)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.EqualValues(t, 1, calls.Load())
	assert.Empty(t, *delays)
}

func TestRoundTrip_GivesUpAfterMaxAttempts(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusBadGateway)
	client, delays := newClient(t)

	resp, err := get(t, client, srv.URL)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.EqualValues(t, DefaultMaxAttempts, calls.Load())
	assert.Len(t, *delays, DefaultMaxAttempts-1)
}

func TestRoundTrip_ConnectionRefusedIsRetriedThenReported(t *testing.T) {
	t.Parallel()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	client, delays := newClient(t)

	resp, err := get(t, client, "http://"+addr+"/")
	if resp != nil {
		resp.Body.Close()
	}
	require.Error(t, err)

	var opErr *net.OpError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, "dial", opErr.Op)
	assert.True(t, RetryableError(err), "got %v", err)
	assert.Len(t, *delays, DefaultMaxAttempts-1)
}

func TestRoundTrip_StopsWhenContextIsCanceled(t *testing.T) {
	t.Parallel()
	srv, calls := statusServer(t, http.StatusServiceUnavailable)
	ctx, cancel := context.WithCancel(context.Background())
	tr := New(http.DefaultTransport.(*http.Transport).Clone())
	tr.sleep = func(ctx context.Context, d time.Duration) error {
		cancel() // the user gives up during the backoff
		return ctx.Err()
	}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	require.Error(t, err)

	assert.True(t, errors.Is(err, context.Canceled), "got %v", err)
	assert.EqualValues(t, 1, calls.Load())
}

func TestReplayable(t *testing.T) {
	t.Parallel()
	mk := func(ctx context.Context, method string, body io.Reader) *http.Request {
		req, err := http.NewRequestWithContext(ctx, method, "http://example.invalid/", body)
		require.NoError(t, err)
		return req
	}
	bg := context.Background()
	unrewindable := io.NopCloser(strings.NewReader("stream"))

	assert.True(t, Replayable(mk(bg, http.MethodGet, nil)))
	assert.True(t, Replayable(mk(bg, http.MethodHead, nil)))
	assert.True(t, Replayable(mk(bg, http.MethodGet, strings.NewReader("body"))), "NewRequest sets GetBody for known readers")
	assert.False(t, Replayable(mk(bg, http.MethodGet, unrewindable)), "a body that cannot be rewound is not replayable")
	assert.False(t, Replayable(mk(bg, http.MethodPost, strings.NewReader("body"))))
	assert.False(t, Replayable(mk(bg, http.MethodDelete, nil)))
	assert.True(t, Replayable(mk(MarkIdempotent(bg), http.MethodPost, strings.NewReader("query"))))
	assert.False(t, Replayable(mk(MarkIdempotent(bg), http.MethodPost, unrewindable)))
}

func TestRetryableError(t *testing.T) {
	t.Parallel()
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: &os.SyscallError{Syscall: "connect", Err: errnoRefused}}
	reset := &net.OpError{Op: "read", Net: "tcp", Err: errnoReset}
	timeout := &net.OpError{Op: "dial", Net: "tcp", Err: &timeoutErr{}}

	type testCase struct {
		name string
		err  error
		want bool
	}
	cases := []testCase{
		{"nil", nil, false},
		{"canceled", context.Canceled, false},
		{"wrapped deadline", fmt.Errorf("x: %w", context.DeadlineExceeded), false},
		{"connection refused", refused, true},
		{"connection refused via url.Error", &url.Error{Op: "Get", URL: "http://x", Err: refused}, true},
		{"connection reset", reset, true},
		{"dial timeout", timeout, true},
		{"op error with unknown cause", &net.OpError{Op: "dial", Err: errors.New("no route")}, false},
		{"EOF before response", io.EOF, true},
		{"unexpected EOF via url.Error", &url.Error{Op: "Get", URL: "http://x", Err: io.ErrUnexpectedEOF}, true},
		{"temporary DNS", &net.DNSError{IsTemporary: true}, true},
		{"DNS not found", &net.DNSError{IsNotFound: true}, false},
		{"filesystem errno is not a network error", &fs.PathError{Op: "stat", Path: "/etc/hosts/foo", Err: syscall.ENOTDIR}, false},
		{"bare errno is not enough", errnoReset, false},
		{"http2 GOAWAY text", errors.New("http2: server sent GOAWAY and closed the connection; LastStreamID=1"), true},
		{"unrelated", errors.New("boom"), false},
	}
	// Every listed errno counts, in the shape the net package reports it.
	for _, errno := range connErrnos {
		cases = append(cases, testCase{
			fmt.Sprintf("errno %d", uintptr(errno)),
			&net.OpError{Op: "read", Net: "tcp", Err: &os.SyscallError{Syscall: "read", Err: errno}},
			true,
		})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, RetryableError(tc.err))
		})
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestRetryAfter(t *testing.T) {
	t.Parallel()
	get := func(v string) (time.Duration, bool) {
		h := http.Header{}
		if v != "" {
			h.Set("Retry-After", v)
		}
		return retryAfter(h)
	}

	d, ok := get("2")
	assert.True(t, ok)
	assert.Equal(t, 2*time.Second, d)

	d, ok = get(time.Now().Add(3 * time.Second).UTC().Format(http.TimeFormat))
	assert.True(t, ok)
	assert.InDelta(t, float64(3*time.Second), float64(d), float64(1500*time.Millisecond))

	// Long waits are reported, not dropped, so the caller can refuse them.
	d, ok = get("3600")
	assert.True(t, ok)
	assert.Equal(t, time.Hour, d)

	d, ok = get("99999999999999999999")
	assert.True(t, ok, "an out-of-range value is a very long wait")
	assert.Greater(t, d, maxRetryAfter)

	d, ok = get("0")
	assert.True(t, ok)
	assert.Zero(t, d)

	d, ok = get(time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat))
	assert.True(t, ok)
	assert.Negative(t, d)

	for _, v := range []string{"", "-1", "1.5", "garbage"} {
		_, ok := get(v)
		assert.False(t, ok, "Retry-After %q", v)
	}
}

func TestBackoffIsBoundedAndJittered(t *testing.T) {
	t.Parallel()
	tr := &Transport{BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}
	for attempt := 1; attempt <= 8; attempt++ {
		want := min(100*time.Millisecond<<(attempt-1), time.Second)
		d := tr.backoff(attempt)
		assert.GreaterOrEqual(t, d, want*3/4, "attempt %d", attempt)
		assert.LessOrEqual(t, d, want*5/4, "attempt %d", attempt)
	}
}
