package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout_FiresBeforeHandlerCompletes(t *testing.T) {
	var handlerRunning bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerRunning = true
		time.Sleep(200 * time.Millisecond)
	})

	wrapped := Timeout(50 * time.Millisecond)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if !handlerRunning {
		t.Error("Handler should have started")
	}

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("Expected status %d, got %d", http.StatusGatewayTimeout, w.Code)
	}
}

func TestTimeout_HandlerCompletesBeforeTimeout(t *testing.T) {
	var handlerCompleted bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCompleted = true
		w.Write([]byte("OK"))
	})

	wrapped := Timeout(100 * time.Millisecond)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if !handlerCompleted {
		t.Error("Handler should have completed")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestTimeout_ContextPropagation(t *testing.T) {
	done := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			close(done)
		case <-time.After(time.Second):
		}
	})

	wrapped := Timeout(50 * time.Millisecond)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have been cancelled within timeout")
	}

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("Expected status %d, got %d", http.StatusGatewayTimeout, w.Code)
	}
}
