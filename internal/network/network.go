package network

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"syscall"
)

// IsOfflineError checks if an error indicates the user is offline
func IsOfflineError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common network error patterns
	errorStr := strings.ToLower(err.Error())
	
	// Common offline indicators
	offlinePatterns := []string{
		"no such host",
		"connection refused",
		"network is unreachable",
		"no route to host",
		"host is down",
		"connection timed out",
		"dial tcp: no route to host",
		"dial tcp: network is unreachable",
		"temporary failure in name resolution",
	}

	for _, pattern := range offlinePatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	// Check for specific error types
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && !dnsErr.Temporary() {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" || opErr.Op == "read" {
			return true
		}
		
		if syscallErr, ok := opErr.Err.(*net.AddrError); ok {
			return syscallErr.Err == "no such host"
		}
		
		if syscallErr, ok := opErr.Err.(syscall.Errno); ok {
			return syscallErr == syscall.ECONNREFUSED || 
				   syscallErr == syscall.ENETUNREACH ||
				   syscallErr == syscall.EHOSTUNREACH
		}
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return IsOfflineError(urlErr.Err)
	}

	return false
}

// GetFriendlyOfflineMessage returns a friendly message for offline scenarios
func GetFriendlyOfflineMessage() string {
	return "It seems you're offline, no donuts for you"
}