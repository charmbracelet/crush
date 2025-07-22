package network

import (
	"errors"
	"net"
	"net/url"
	"syscall"
	"testing"
)

func TestIsOfflineError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "no such host error",
			err:      errors.New("dial tcp: lookup example.com: no such host"),
			expected: true,
		},
		{
			name:     "connection refused error",
			err:      errors.New("dial tcp 127.0.0.1:80: connection refused"),
			expected: true,
		},
		{
			name:     "network unreachable error",
			err:      errors.New("dial tcp: network is unreachable"),
			expected: true,
		},
		{
			name:     "timeout error",
			err:      &net.OpError{Op: "dial", Err: syscall.ETIMEDOUT},
			expected: true,
		},
		{
			name:     "connection refused syscall",
			err:      &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
			expected: true,
		},
		{
			name:     "url error with underlying network error",
			err:      &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("no such host")},
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "server error (not offline)",
			err:      errors.New("500 internal server error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOfflineError(tt.err)
			if result != tt.expected {
				t.Errorf("IsOfflineError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestGetFriendlyOfflineMessage(t *testing.T) {
	msg := GetFriendlyOfflineMessage()
	expected := "It seems you're offline, no donuts for you"
	if msg != expected {
		t.Errorf("GetFriendlyOfflineMessage() = %q, expected %q", msg, expected)
	}
}