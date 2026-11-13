//go:build windows

package httpretry

import "golang.org/x/sys/windows"

// The errnos a refused and a reset TCP connection report on this platform.
const (
	errnoRefused = windows.WSAECONNREFUSED
	errnoReset   = windows.WSAECONNRESET
)
