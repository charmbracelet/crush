package server

import (
	"errors"
	"io/fs"
	"net"
	"syscall"
)

// IsStaleSocketErr reports whether err indicates that a Unix-domain
// socket file exists on disk but no process is listening on it (a stale
// or orphaned socket). It returns false for nil and for timeout errors.
//
// The classification is cross-platform: ECONNREFUSED and fs.ErrNotExist
// are defined on every supported OS, so callers on Windows can use the
// same helper even though stale-socket recovery only applies to Unix
// sockets in practice.
func IsStaleSocketErr(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, fs.ErrNotExist)
}
