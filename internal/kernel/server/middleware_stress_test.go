package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimeout_Stress_ConcurrentRequests(t *testing.T) {
	var handlerCallCount int32
	var concurrentHandlers int32
	var maxConcurrent int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&concurrentHandlers, 1)
		oldMax := atomic.LoadInt32(&maxConcurrent)
		if cur > oldMax {
			atomic.CompareAndSwapInt32(&maxConcurrent, oldMax, cur)
		}

		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&handlerCallCount, 1)
		atomic.AddInt32(&concurrentHandlers, -1)

		w.Write([]byte("OK"))
	})

	wrapped := Timeout(50 * time.Millisecond)(handler)

	var wg sync.WaitGroup
	requestCount := 100

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)
		}()
	}

	wg.Wait()

	t.Logf("Total requests: %d, Handler calls: %d, Max concurrent: %d", requestCount, handlerCallCount, maxConcurrent)

	if handlerCallCount != int32(requestCount) {
		t.Errorf("Expected all handlers to complete, got %d/%d", handlerCallCount, requestCount)
	}
}

func TestTimeout_Stress_RapidTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.Write([]byte("OK"))
	})

	wrapped := Timeout(1 * time.Millisecond)(handler)

	var wg sync.WaitGroup
	requestCount := 50

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)
		}()
	}

	wg.Wait()

	t.Logf("Completed %d rapid timeout requests", requestCount)
}

func TestTimeout_Stress_MixedTimeout(t *testing.T) {
	var shortHandlerCalls int32
	var longHandlerCalls int32

	shortHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&shortHandlerCalls, 1)
		w.Write([]byte("short"))
	})

	longHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&longHandlerCalls, 1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("long"))
	})

	shortWrapped := Timeout(100 * time.Millisecond)(shortHandler)
	longWrapped := Timeout(50 * time.Millisecond)(longHandler)

	// Collect status codes to avoid race on shared variable
	var longHandlerStatusCodes []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/short", nil)
			w := httptest.NewRecorder()
			shortWrapped.ServeHTTP(w, req)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/long", nil)
			w := httptest.NewRecorder()
			longWrapped.ServeHTTP(w, req)
			mu.Lock()
			longHandlerStatusCodes = append(longHandlerStatusCodes, w.Code)
			mu.Unlock()
		}()
	}

	wg.Wait()

	t.Logf("Short handler calls: %d, Long handler calls: %d", shortHandlerCalls, longHandlerCalls)
	t.Logf("Long handler status codes sample: %v", longHandlerStatusCodes[:min(5, len(longHandlerStatusCodes))])

	if shortHandlerCalls != 50 {
		t.Errorf("Expected 50 short handler calls, got %d", shortHandlerCalls)
	}

	// Verify long handler timed out (504) - all should be 504 since timeout < handler duration
	for i, code := range longHandlerStatusCodes {
		if code != http.StatusGatewayTimeout {
			t.Errorf("Long handler request %d returned %d, expected 504", i, code)
		}
	}
}

func TestTimeout_ContextCancellation(t *testing.T) {
	done := make(chan struct{})
	started := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
			close(done)
		case <-time.After(5 * time.Second):
		}
	})

	wrapped := Timeout(50 * time.Millisecond)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	go func() {
		<-started
		time.Sleep(10 * time.Millisecond)
	}()

	wrapped.ServeHTTP(w, req)

	select {
	case <-done:
		t.Log("Context was properly cancelled")
	case <-time.After(500 * time.Millisecond):
		t.Error("Context was not cancelled within timeout")
	}
}

func TestTimeout_ZeroDuration(t *testing.T) {
	var handlerCalled int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&handlerCalled, 1)
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte("OK"))
	})

	wrapped := Timeout(0)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	wrapped.ServeHTTP(w, req)
	elapsed := time.Since(start)

	t.Logf("Zero timeout elapsed: %v, status: %d, handler called: %d", elapsed, w.Code, handlerCalled)

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("Expected 504 timeout status, got %d", w.Code)
	}

}

func BenchmarkTimeout_Middleware(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte("OK"))
	})

	wrapped := Timeout(100 * time.Millisecond)(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}
}
