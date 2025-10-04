package volley

import (
	"errors"
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	tests := []struct {
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{0, 750 * time.Millisecond, 1500 * time.Millisecond},  // 1s ± 25% jitter
		{1, 1500 * time.Millisecond, 3000 * time.Millisecond}, // 2s ± 25% jitter
		{2, 3 * time.Second, 6 * time.Second},                 // 4s ± 25% jitter
		{3, 6 * time.Second, 12 * time.Second},                // 8s ± 25% jitter
		{10, 45 * time.Second, 75 * time.Second},              // capped at 60s ± 25% jitter
	}

	for _, tt := range tests {
		delay := retryDelay(tt.attempt)
		if delay < tt.minExpected || delay > tt.maxExpected {
			t.Errorf("retryDelay(%d) = %v, want between %v and %v",
				tt.attempt, delay, tt.minExpected, tt.maxExpected)
		}
	}
}

func TestRetryDelayForError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{
			name:        "rate limit error - first attempt",
			err:         errors.New("429 rate limit exceeded"),
			attempt:     0,
			minExpected: 3750 * time.Millisecond, // 5s ± 25%
			maxExpected: 6250 * time.Millisecond,
		},
		{
			name:        "rate limit error - second attempt",
			err:         errors.New("rate limit exceeded"),
			attempt:     1,
			minExpected: 7500 * time.Millisecond, // 10s ± 25%
			maxExpected: 12500 * time.Millisecond,
		},
		{
			name:        "timeout error - first attempt",
			err:         errors.New("context deadline exceeded"),
			attempt:     0,
			minExpected: 1500 * time.Millisecond, // 2s ± 25%
			maxExpected: 2500 * time.Millisecond,
		},
		{
			name:        "network error - first attempt",
			err:         errors.New("connection refused"),
			attempt:     0,
			minExpected: 375 * time.Millisecond, // 500ms ± 25%
			maxExpected: 625 * time.Millisecond,
		},
		{
			name:        "network error - third attempt",
			err:         errors.New("network error"),
			attempt:     2,
			minExpected: 1500 * time.Millisecond, // 2s ± 25%
			maxExpected: 2500 * time.Millisecond,
		},
		{
			name:        "unknown error - first attempt",
			err:         errors.New("something went wrong"),
			attempt:     0,
			minExpected: 750 * time.Millisecond, // 1s ± 25%
			maxExpected: 1250 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := retryDelayForError(tt.err, tt.attempt)
			if delay < tt.minExpected || delay > tt.maxExpected {
				t.Errorf("retryDelayForError(%v, %d) = %v, want between %v and %v",
					tt.err, tt.attempt, delay, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorClass
	}{
		{"nil error", nil, ErrorClassUnknown},
		{"rate limit 429", errors.New("429 Too Many Requests"), ErrorClassRateLimit},
		{"rate limit text", errors.New("rate limit exceeded"), ErrorClassRateLimit},
		{"auth 401", errors.New("401 unauthorized"), ErrorClassAuth},
		{"auth 403", errors.New("403 forbidden"), ErrorClassAuth},
		{"auth text", errors.New("unauthorized access"), ErrorClassAuth},
		{"validation 400", errors.New("400 bad request"), ErrorClassValidation},
		{"validation text", errors.New("invalid input"), ErrorClassValidation},
		{"timeout", errors.New("context deadline exceeded"), ErrorClassTimeout},
		{"timeout text", errors.New("request timeout"), ErrorClassTimeout},
		{"network connection", errors.New("connection refused"), ErrorClassNetwork},
		{"network text", errors.New("network error"), ErrorClassNetwork},
		{"unknown", errors.New("something went wrong"), ErrorClassUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyError(tt.err)
			if got != tt.expected {
				t.Errorf("classifyError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
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
