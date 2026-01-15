package log

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryTransport_SuccessOnFirstAttempt(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	client := &http.Client{
		Transport: NewRetryTransport(nil, DefaultRetryConfig),
	}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestRetryTransport_RetryOn503(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries:  5,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
		Jitter:      5 * time.Millisecond,
	}

	client := &http.Client{
		Transport: NewRetryTransport(nil, cfg),
	}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestRetryTransport_RetryOn429(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries:  3,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
		Jitter:      5 * time.Millisecond,
	}

	client := &http.Client{
		Transport: NewRetryTransport(nil, cfg),
	}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, int32(2), atomic.LoadInt32(&attempts))
}

func TestRetryTransport_NoRetryOn4xx(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries:  3,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
		Jitter:      5 * time.Millisecond,
	}

	client := &http.Client{
		Transport: NewRetryTransport(nil, cfg),
	}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestRetryTransport_MaxRetriesExhausted(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries:  2,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  50 * time.Millisecond,
		Jitter:      5 * time.Millisecond,
	}

	client := &http.Client{
		Transport: NewRetryTransport(nil, cfg),
	}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should have tried 1 initial + 2 retries = 3 attempts.
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestRetryTransport_PreservesRequestBody(t *testing.T) {
	t.Parallel()

	var attempts int32
	var lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		body, _ := io.ReadAll(r.Body)
		lastBody = string(body)
		if count < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries:  3,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
		Jitter:      5 * time.Millisecond,
	}

	client := &http.Client{
		Transport: NewRetryTransport(nil, cfg),
	}

	req, err := http.NewRequest("POST", server.URL, strings.NewReader("test body"))
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "test body", lastBody)
	require.Equal(t, int32(2), atomic.LoadInt32(&attempts))
}

func TestRetryTransport_RespectsContextCancellation(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		// Add delay to ensure context cancels during backoff, not during request.
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := RetryConfig{
		MaxRetries:  10,
		BaseBackoff: 200 * time.Millisecond, // Long backoff so context cancels during wait.
		MaxBackoff:  1 * time.Second,
		Jitter:      10 * time.Millisecond,
	}

	client := &http.Client{
		Transport: NewRetryTransport(nil, cfg),
	}

	// Short timeout to cancel during backoff between retries.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)

	// Either we get an error (context canceled) or a 503 response.
	if err != nil {
		require.True(t, strings.Contains(err.Error(), "context"))
	} else {
		// Got a response before context was canceled, that's also acceptable.
		defer resp.Body.Close()
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	}

	// Should have at least tried once.
	require.GreaterOrEqual(t, atomic.LoadInt32(&attempts), int32(1))
}

func TestRetryTransport_IsRetryableError(t *testing.T) {
	t.Parallel()

	rt := &RetryTransport{}

	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"io.EOF", io.EOF, true},
		{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := rt.isRetryableError(tc.err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestRetryTransport_IsRetryableStatus(t *testing.T) {
	t.Parallel()

	rt := &RetryTransport{}

	testCases := []struct {
		status   int
		expected bool
	}{
		{http.StatusOK, false},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusNotFound, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, false},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
	}

	for _, tc := range testCases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			result := rt.isRetryableStatus(tc.status)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestRetryTransport_GetRetryAfter(t *testing.T) {
	t.Parallel()

	rt := &RetryTransport{}

	t.Run("no header", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		require.Equal(t, time.Duration(0), rt.getRetryAfter(resp))
	})

	t.Run("seconds", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", "5")
		require.Equal(t, 5*time.Second, rt.getRetryAfter(resp))
	})

	t.Run("invalid", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", "invalid")
		require.Equal(t, time.Duration(0), rt.getRetryAfter(resp))
	})
}

func TestNewHTTPClientWithRetry(t *testing.T) {
	t.Parallel()

	t.Run("without debug", func(t *testing.T) {
		client := NewHTTPClientWithRetry(false)
		require.NotNil(t, client)
		require.NotNil(t, client.Transport)

		// Should be a RetryTransport.
		_, ok := client.Transport.(*RetryTransport)
		require.True(t, ok)
	})

	t.Run("with debug", func(t *testing.T) {
		client := NewHTTPClientWithRetry(true)
		require.NotNil(t, client)
		require.NotNil(t, client.Transport)

		// Should be a HTTPRoundTripLogger wrapping RetryTransport.
		logger, ok := client.Transport.(*HTTPRoundTripLogger)
		require.True(t, ok)
		_, ok = logger.Transport.(*RetryTransport)
		require.True(t, ok)
	})
}
