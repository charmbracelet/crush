//go:build windows

package httpretry

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// connErrnos are the connection-level errors worth a second attempt.
// Windows sockets report WSA codes, and some paths the matching Win32
// codes. The POSIX names in package syscall are placeholders on Windows
// that no socket call returns, and syscall.Errno.Is does not map between
// them, so they would never match here.
var connErrnos = []syscall.Errno{
	windows.WSAECONNRESET,
	windows.WSAECONNREFUSED,
	windows.WSAECONNABORTED,
	windows.WSAEHOSTUNREACH,
	windows.WSAENETUNREACH,
	windows.ERROR_NETNAME_DELETED, // a reset connection, as Go's poller treats it
	windows.ERROR_CONNECTION_REFUSED,
	windows.ERROR_CONNECTION_ABORTED,
	windows.ERROR_HOST_UNREACHABLE,
	windows.ERROR_NETWORK_UNREACHABLE,
}
