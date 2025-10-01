package volley

import (
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	tests := []struct {
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{0, 1 * time.Second, 2 * time.Second},
		{1, 2 * time.Second, 3 * time.Second},
		{2, 4 * time.Second, 5 * time.Second},
		{3, 8 * time.Second, 9 * time.Second},
		{10, 60 * time.Second, 61 * time.Second}, // capped at max
	}

	for _, tt := range tests {
		delay := retryDelay(tt.attempt)
		if delay < tt.minExpected || delay > tt.maxExpected {
			t.Errorf("retryDelay(%d) = %v, want between %v and %v",
				tt.attempt, delay, tt.minExpected, tt.maxExpected)
		}
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name       string
		err        string
		attempt    int
		maxRetries int
		want       bool
	}{
		{"rate limit", "429 rate limit exceeded", 0, 3, true},
		{"timeout", "context deadline exceeded", 1, 3, true},
		{"network error", "connection refused", 0, 3, true},
		{"auth error", "401 unauthorized", 0, 3, false},
		{"bad request", "400 bad request", 0, 3, false},
		{"max retries reached", "429 rate limit", 3, 3, false},
		{"unknown error", "something went wrong", 0, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &testError{msg: tt.err}
			got := shouldRetry(err, tt.attempt, tt.maxRetries)
			if got != tt.want {
				t.Errorf("shouldRetry(%q, %d, %d) = %v, want %v",
					tt.err, tt.attempt, tt.maxRetries, got, tt.want)
			}
		})
	}
}

func TestDefaultVolleyOptions(t *testing.T) {
	opts := DefaultVolleyOptions()

	if opts.MaxConcurrent <= 0 {
		t.Error("MaxConcurrent should be positive")
	}

	if opts.MaxRetries < 0 {
		t.Error("MaxRetries should be non-negative")
	}

	if !opts.ShowProgress {
		t.Error("ShowProgress should be true by default")
	}

	if opts.OutputFormat != "text" {
		t.Errorf("OutputFormat = %q, want %q", opts.OutputFormat, "text")
	}
}

func TestTaskStatus(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusSuccess,
		TaskStatusFailed,
		TaskStatusCanceled,
	}

	// Just verify they're defined
	for _, status := range statuses {
		if status == "" {
			t.Error("TaskStatus should not be empty")
		}
	}
}

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
