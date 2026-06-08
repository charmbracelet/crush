//go:build !windows

package server

import (
	"errors"
	"io/fs"
	"net"
	"syscall"
)

// isStaleSocketErr reports whether err indicates a Unix-domain socket file
// exists on disk but no process is listening on it (a stale or orphaned
// socket). It returns false for nil and for timeout errors.
func isStaleSocketErr(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, fs.ErrNotExist)
}
