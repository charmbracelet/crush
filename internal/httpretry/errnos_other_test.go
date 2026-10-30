//go:build !windows

package httpretry

import "syscall"

// The errnos a refused and a reset TCP connection report on this platform.
const (
	errnoRefused = syscall.ECONNREFUSED
	errnoReset   = syscall.ECONNRESET
)
