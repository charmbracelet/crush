package agent

import (
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================
// Mock RoundTripper for testing
// ============================================================

// mockRoundTripper is a test double that intercepts requests
// and can simulate failures and delays.
type mockRoundTripper struct {
	// Response settings
	statusCode          int
	responseBody        string
	delay               time.Duration
	eventualSuccessCode int // If > 0, return this after maxFails exhausted

	// Call tracking
	mu          sync.Mutex
	callCount   int
	lastRequest *http.Request
	failCount   int
	maxFails    int
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.callCount++
	m.lastRequest = req

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	// Fail with statusCode for first maxFails attempts
	if m.maxFails > 0 && m.failCount < m.maxFails {
		m.failCount++
		m.mu.Unlock()
		return &http.Response{
			StatusCode: m.statusCode,
			Body:       io.NopCloser(nil),
		}, nil
	}
	m.mu.Unlock()

	// After maxFails, return eventualSuccessCode if set, else statusCode
	finalStatus := m.statusCode
	if m.eventualSuccessCode > 0 {
		finalStatus = m.eventualSuccessCode
	}
	return &http.Response{
		StatusCode: finalStatus,
		Body:       io.NopCloser(nil),
		Header:     http.Header{},
	}, nil
}

func (m *mockRoundTripper) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// ============================================================
// retryTransport Stress Tests
// ============================================================

func TestRetryTransport_SuccessAfterRetries(t *testing.T) {
	mock := &mockRoundTripper{
		statusCode:          http.StatusInternalServerError, // Fail with 500
		eventualSuccessCode: http.StatusOK,                  // Success on 3rd attempt
		maxFails:            2,                              // Fail first 2 attempts
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 3,
	}

	req, _ := http.NewRequest("GET", "http://test/models", nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if mock.getCallCount() != 3 {
		t.Errorf("expected 3 attempts, got %d", mock.getCallCount())
	}
}

func TestRetryTransport_AllRetriesFail(t *testing.T) {
	mock := &mockRoundTripper{
		statusCode: http.StatusInternalServerError,
		maxFails:   100, // Fail all attempts
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 3,
	}

	req, _ := http.NewRequest("GET", "http://test/models", nil)
	resp, err := transport.RoundTrip(req)

	// When retries exhaust, should return error (not nil err with 500 response)
	if err == nil {
		t.Fatalf("expected error after exhausting retries, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response after exhausting retries, got resp with status %d", resp.StatusCode)
	}
	if mock.getCallCount() != 4 { // initial + 3 retries
		t.Errorf("expected 4 attempts, got %d", mock.getCallCount())
	}
}

func TestRetryTransport_RateLimitRetry(t *testing.T) {
	mock := &mockRoundTripper{
		statusCode:          http.StatusTooManyRequests,
		eventualSuccessCode: http.StatusOK,
		maxFails:            2, // Fail first 2 with 429
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 3,
	}

	req, _ := http.NewRequest("GET", "http://test/models", nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if mock.getCallCount() != 3 {
		t.Errorf("expected 3 attempts, got %d", mock.getCallCount())
	}
}

func TestRetryTransport_NoRetryOn4xx(t *testing.T) {
	mock := &mockRoundTripper{
		statusCode: http.StatusBadRequest,
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 3,
	}

	req, _ := http.NewRequest("GET", "http://test/models", nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	if mock.getCallCount() != 1 {
		t.Errorf("expected 1 attempt (no retry on 4xx), got %d", mock.getCallCount())
	}
}

func TestRetryTransport_ConcurrentRequests(t *testing.T) {
	var totalRequests int32
	var maxConcurrent int32
	var concurrentMu sync.Mutex
	var currentConcurrent int32

	mock := &mockRoundTripper{
		statusCode: http.StatusOK,
		delay:      10 * time.Millisecond,
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 0, // No retries to simplify test
	}

	var wg sync.WaitGroup
	requestCount := 50

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			concurrentMu.Lock()
			currentConcurrent++
			if currentConcurrent > maxConcurrent {
				maxConcurrent = currentConcurrent
			}
			concurrentMu.Unlock()

			req, _ := http.NewRequest("GET", "http://test/models", nil)
			transport.RoundTrip(req)

			concurrentMu.Lock()
			currentConcurrent--
			concurrentMu.Unlock()

			atomic.AddInt32(&totalRequests, 1)
		}()
	}

	wg.Wait()

	t.Logf("Total requests: %d, Max concurrent: %d", totalRequests, maxConcurrent)

	if totalRequests != int32(requestCount) {
		t.Errorf("expected %d requests, got %d", requestCount, totalRequests)
	}
}

func TestRetryTransport_ExponentialBackoff(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow backoff test in short mode")
	}

	var attemptTimes []time.Time
	mu := sync.Mutex{}

	mock := &mockRoundTripper{
		statusCode: http.StatusInternalServerError,
	}

	// Wrap to capture timing
	wrappedTransport := &timingTransport{
		transport:    mock,
		attemptTimes: &attemptTimes,
		mu:           &mu,
	}

	transport := &retryTransport{
		transport:  wrappedTransport,
		maxRetries: 3,
	}

	req, _ := http.NewRequest("GET", "http://test/models", nil)
	transport.RoundTrip(req)

	mu.Lock()
	if len(attemptTimes) != 4 {
		t.Fatalf("expected 4 attempts, got %d", len(attemptTimes))
	}
	mu.Unlock()

	// Check backoff intervals: 1s, 2s, 4s (capped at 10s)
	time.Sleep(100 * time.Millisecond) // Let last timing complete

	mu.Lock()
	backoff0to1 := attemptTimes[1].Sub(attemptTimes[0])
	backoff1to2 := attemptTimes[2].Sub(attemptTimes[1])
	backoff2to3 := attemptTimes[3].Sub(attemptTimes[2])
	mu.Unlock()

	t.Logf("Backoff intervals: 0->1: %v, 1->2: %v, 2->3: %v (with jitter)",
		backoff0to1, backoff1to2, backoff2to3)

	// With ±50% jitter: 0.5-1.5s for 1s base, 1-3s for 2s base, 2-6s for 4s base
	if backoff0to1 < 500*time.Millisecond || backoff0to1 > 1500*time.Millisecond {
		t.Errorf("expected 0.5-1.5s backoff (with ±50%% jitter), got %v", backoff0to1)
	}
	if backoff1to2 < 1000*time.Millisecond || backoff1to2 > 3000*time.Millisecond {
		t.Errorf("expected 1-3s backoff (with ±50%% jitter), got %v", backoff1to2)
	}
	if backoff2to3 < 2000*time.Millisecond || backoff2to3 > 6000*time.Millisecond {
		t.Errorf("expected 2-6s backoff (with ±50%% jitter), got %v", backoff2to3)
	}
}

// timingTransport wraps a transport to record attempt timestamps
type timingTransport struct {
	transport    *mockRoundTripper
	attemptTimes *[]time.Time
	mu           *sync.Mutex
}

func (t *timingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	*t.attemptTimes = append(*t.attemptTimes, time.Now())
	t.mu.Unlock()
	return t.transport.RoundTrip(req)
}

func TestRetryTransport_NetworkErrorRetries(t *testing.T) {
	mock := &mockRoundTripper{
		statusCode: http.StatusInternalServerError,
		maxFails:   2,
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 3,
	}

	req, _ := http.NewRequest("GET", "http://test/models", nil)
	transport.RoundTrip(req)

	// Should retry on failures
	if mock.getCallCount() < 2 {
		t.Errorf("expected retries on failure, got %d attempts", mock.getCallCount())
	}
}

// ============================================================
// newRetryHTTPClient Tests
// ============================================================

func TestNewRetryHTTPClient_Basic(t *testing.T) {
	// Just verify client creation works
	client := newRetryHTTPClient(3)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 120*time.Second {
		t.Errorf("expected 120s timeout, got %v", client.Timeout)
	}
}

func TestNewRetryHTTPClient_HasTimeout(t *testing.T) {
	client := newRetryHTTPClient(3)
	if client.Timeout != 120*time.Second {
		t.Errorf("expected 120s timeout, got %v", client.Timeout)
	}
}

func TestNewRetryHTTPClient_ZeroRetries(t *testing.T) {
	mock := &mockRoundTripper{
		statusCode: http.StatusInternalServerError,
		maxFails:   1,
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 0,
	}

	req, _ := http.NewRequest("GET", "http://test/models", nil)
	transport.RoundTrip(req)

	// With 0 retries, should only make 1 attempt
	if mock.getCallCount() != 1 {
		t.Errorf("expected 1 attempt with 0 retries, got %d", mock.getCallCount())
	}
}

func TestNewRetryHTTPClient_Concurrent(t *testing.T) {
	var requestCount int32
	var mu sync.Mutex

	mock := &mockRoundTripper{
		statusCode: http.StatusOK,
		delay:      5 * time.Millisecond,
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 2,
	}

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", "http://test/models", nil)
			transport.RoundTrip(req)
			mu.Lock()
			requestCount++
			mu.Unlock()
		}()
	}

	wg.Wait()

	mu.Lock()
	if requestCount != 20 {
		t.Errorf("expected 20 requests, got %d", requestCount)
	}
	mu.Unlock()
}

// ============================================================
// Stress Tests
// ============================================================

func TestRetryStress_ManyRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	mock := &mockRoundTripper{
		statusCode:          http.StatusInternalServerError,
		eventualSuccessCode: http.StatusOK,
		maxFails:            9, // Fail first 9, succeed on 10th
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 10,
	}

	req, _ := http.NewRequest("GET", "http://test/models", nil)
	start := time.Now()
	resp, err := transport.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	t.Logf("Succeeded after %d attempts in %v", mock.getCallCount(), elapsed)
}

func TestRetryStress_HighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	var totalRequests int64
	var errors int64
	var mu sync.Mutex

	mock := &mockRoundTripper{
		statusCode: http.StatusOK,
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 2,
	}

	var wg sync.WaitGroup
	requestCount := 100

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", "http://test/models", nil)
			resp, err := transport.RoundTrip(req)
			mu.Lock()
			totalRequests++
			if err != nil || resp.StatusCode >= 500 {
				errors++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	mu.Lock()
	successRate := float64(requestCount-int(errors)) / float64(requestCount) * 100
	t.Logf("Total requests: %d, Errors: %d, Success rate: %.1f%%",
		totalRequests, errors, successRate)
	mu.Unlock()
}

// ============================================================
// Edge Cases
// ============================================================

func TestRetryTransport_NilBodyOnRetry(t *testing.T) {
	mock := &mockRoundTripper{
		statusCode:          http.StatusInternalServerError,
		eventualSuccessCode: http.StatusOK,
		maxFails:            1,
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 3,
	}

	req, _ := http.NewRequest("GET", "http://test/models", nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRetryTransport_EmptyPath(t *testing.T) {
	transport := &retryTransport{
		transport:  http.DefaultTransport,
		maxRetries: 1,
	}

	req, _ := http.NewRequest("GET", "", nil)
	_, err := transport.RoundTrip(req)

	// Should handle empty URL gracefully
	if err == nil {
		// May succeed with default transport
	}
}

func TestRetryTransport_RequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mock := &mockRoundTripper{
		statusCode: http.StatusOK,
		delay:      100 * time.Millisecond,
	}

	transport := &retryTransport{
		transport:  mock,
		maxRetries: 10,
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://test/models", nil)

	// Cancel after first attempt
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	transport.RoundTrip(req)

	// Note: The retry transport doesn't currently check context cancellation
	// between retries, so this test documents current behavior
}
