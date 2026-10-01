//go:build !windows

package httpretry

import "syscall"

// connErrnos are the connection-level errors worth a second attempt. They
// are listed explicitly rather than derived from net.Error's Temporary or
// Timeout methods, which syscall.Errno implements for every value.
var connErrnos = []syscall.Errno{
	syscall.ECONNRESET,
	syscall.ECONNREFUSED,
	syscall.ECONNABORTED,
	syscall.EPIPE,
	syscall.EHOSTUNREACH,
	syscall.ENETUNREACH,
}
