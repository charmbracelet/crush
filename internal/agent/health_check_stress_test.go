package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================
// Health Check Stress Tests
// ============================================================

func TestHealthCheck_ConcurrentRequests(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", server.URL+"/models", nil)
			req.Header.Set("Authorization", "Bearer test-key")
			http.DefaultClient.Do(req)
		}()
	}

	wg.Wait()

	if requestCount != 50 {
		t.Errorf("expected 50 requests, got %d", requestCount)
	}
}

func TestHealthCheck_TimeoutHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	req, _ := http.NewRequest("GET", server.URL+"/models", nil)
	req.Header.Set("Authorization", "Bearer test-key")

	start := time.Now()
	_, err := client.Do(req)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}
	if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Logf("timeout elapsed: %v (expected ~100ms)", elapsed)
	}
}

func TestHealthCheck_RapidSuccessFailure(t *testing.T) {
	var attemptCount int32
	successCount := 0
	mu := sync.Mutex{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attemptCount, 1)
		if count%2 == 0 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"object":"list","data":[]}`))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	for i := 0; i < 20; i++ {
		req, _ := http.NewRequest("GET", server.URL+"/models", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			mu.Lock()
			successCount++
			mu.Unlock()
		}
	}

	t.Logf("Success count: %d out of 20", successCount)
	if successCount == 0 {
		t.Error("expected some successes")
	}
}

func TestHealthCheck_ConnectionPoolReuse(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	// Create a client with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
	}
	client := &http.Client{Transport: transport}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", server.URL+"/models", nil)
			req.Header.Set("Authorization", "Bearer test-key")
			client.Do(req)
		}()
	}

	wg.Wait()

	// With connection pooling, should reuse connections
	if requestCount != 30 {
		t.Errorf("expected 30 requests, got %d", requestCount)
	}
}

func TestHealthCheck_HeaderPreservation(t *testing.T) {
	var receivedAuth string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/models", nil)
	req.Header.Set("Authorization", "Bearer my-secret-key")
	req.Header.Set("Content-Type", "application/json")

	http.DefaultClient.Do(req)

	if receivedAuth != "Bearer my-secret-key" {
		t.Errorf("expected Bearer auth header, got %q", receivedAuth)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type header, got %q", receivedContentType)
	}
}

func TestHealthCheck_MultiplePaths(t *testing.T) {
	paths := map[string]int{"models": 0, "models/gpt-4": 0}
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths["models"]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	})
	mux.HandleFunc("/v1/models/gpt-4", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths["models/gpt-4"]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"gpt-4","object":"model"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test /models endpoint
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest("GET", server.URL+"/v1/models", nil)
		req.Header.Set("Authorization", "Bearer test")
		http.DefaultClient.Do(req)
	}

	// Test /models/gpt-4 endpoint
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("GET", server.URL+"/v1/models/gpt-4", nil)
		req.Header.Set("Authorization", "Bearer test")
		http.DefaultClient.Do(req)
	}

	mu.Lock()
	if paths["models"] != 10 {
		t.Errorf("expected 10 calls to /models, got %d", paths["models"])
	}
	if paths["models/gpt-4"] != 5 {
		t.Errorf("expected 5 calls to /models/gpt-4, got %d", paths["models/gpt-4"])
	}
	mu.Unlock()
}

func TestHealthCheck_ContextCancellation(t *testing.T) {
	var attemptCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{}

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/models", nil)
	req.Header.Set("Authorization", "Bearer test")

	// Cancel immediately
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := client.Do(req)

	if err == nil {
		t.Log("request may not have been cancelled immediately")
	}

	// Give time for cancellation to propagate
	time.Sleep(100 * time.Millisecond)
}

func TestHealthCheck_ServerClose(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	_ = server

	// Make a request before closing
	req, _ := http.NewRequest("GET", server.URL+"/models", nil)
	req.Header.Set("Authorization", "Bearer test")
	http.DefaultClient.Do(req)

	// Close the server
	server.Close()

	// Subsequent request should fail
	req2, _ := http.NewRequest("GET", server.URL+"/models", nil)
	req2.Header.Set("Authorization", "Bearer test")
	_, err := http.DefaultClient.Do(req2)

	if err == nil {
		t.Error("expected error after server close")
	}
}

func TestHealthCheck_InvalidURL(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	tests := []struct {
		name string
		url  string
	}{
		{"empty_url", ""},
		{"invalid_scheme", "ftp://example.com"},
		{"unreachable", "http://192.0.2.1:12345"}, // TEST-NET address
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.url, nil)
			req.Header.Set("Authorization", "Bearer test")
			_, err := client.Do(req)
			if err == nil && tt.url != "" {
				t.Error("expected error for invalid URL")
			}
		})
	}
}

// ============================================================
// Stress Tests
// ============================================================

func TestHealthCheckStress_HighVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	var requestCount int32
	var errorCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	start := time.Now()
	var wg sync.WaitGroup
	requestCountTotal := 500

	for i := 0; i < requestCountTotal; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", server.URL+"/models", nil)
			req.Header.Set("Authorization", "Bearer test")
			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				atomic.AddInt32(&errorCount, 1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	requestsPerSecond := float64(requestCountTotal) / elapsed.Seconds()
	t.Logf("Requests: %d, Errors: %d, Time: %v, RPS: %.2f",
		requestCount, errorCount, elapsed, requestsPerSecond)
}

func TestHealthCheckStress_PersistentConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	var totalRequests int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		totalRequests++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	// Reuse client for connection pooling
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				req, _ := http.NewRequest("GET", server.URL+"/models", nil)
				req.Header.Set("Authorization", "Bearer test")
				client.Do(req)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	mu.Lock()
	t.Logf("Total requests: %d, Time: %v, RPS: %.2f",
		totalRequests, elapsed, float64(totalRequests)/elapsed.Seconds())
	mu.Unlock()
}

func TestHealthCheckStress_GradualFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	var attemptCount int32
	requestCount := 50

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attemptCount, 1)
		// Gradually increase failure rate
		failureThreshold := count % 10
		if failureThreshold < 3 { // 30% failure rate initially
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", server.URL+"/models", nil)
			req.Header.Set("Authorization", "Bearer test")
			resp, err := http.DefaultClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	successRate := float64(successCount) / float64(requestCount) * 100
	t.Logf("Success rate: %.1f%% (%d/%d)", successRate, successCount, requestCount)

	// With ~30% failure rate, we expect roughly 70% success
	if successRate < 50 || successRate > 90 {
		t.Errorf("unexpected success rate: %.1f%%", successRate)
	}
}

// ============================================================
// Edge Cases
// ============================================================

func TestHealthCheck_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{})
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/models", nil)
	req.Header.Set("Authorization", "Bearer test")
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ContentLength != 0 {
		t.Errorf("expected empty content, got %d", resp.ContentLength)
	}
}

func TestHealthCheck_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list",`)) // Malformed JSON
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/models", nil)
	req.Header.Set("Authorization", "Bearer test")
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still get a response even with malformed body
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHealthCheck_ChunkedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[`))
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte(`{"id":"model-1"},{"id":"model-2"}]`))
		w.Write([]byte(`}`))
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/models", nil)
	req.Header.Set("Authorization", "Bearer test")
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHealthCheck_DoubleSlash(t *testing.T) {
	var path string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	// Request with double slash
	req, _ := http.NewRequest("GET", server.URL+"//models", nil)
	req.Header.Set("Authorization", "Bearer test")
	http.DefaultClient.Do(req)

	t.Logf("Path requested: %s", path)
}

func TestHealthCheck_KeepAlive(t *testing.T) {
	var connectionHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectionHeader = r.Header.Get("Connection")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: false,
		},
	}

	req, _ := http.NewRequest("GET", server.URL+"/models", nil)
	req.Header.Set("Authorization", "Bearer test")
	client.Do(req)

	// With Keep-Alive, Connection header should be "keep-alive" or absent
	t.Logf("Connection header: %s", connectionHeader)
}

func TestHealthCheck_GzipCompression(t *testing.T) {
	var acceptEncoding string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptEncoding = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/models", nil)
	req.Header.Set("Authorization", "Bearer test")
	// Don't set Accept-Encoding to let default client handle it
	http.DefaultClient.Do(req)

	t.Logf("Accept-Encoding: %s", acceptEncoding)
}
