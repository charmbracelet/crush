//go:build !windows

package server

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"time"
)

// staleSocketDialTimeout bounds the probe used to detect whether a Unix
// socket file on disk is backed by a live listener.
const staleSocketDialTimeout = 200 * time.Millisecond

// listen binds a net.Listener on the given network and address.
//
// For unix sockets it self-heals from stale socket files: if the path
// already exists on disk, it first probes with a short net.DialTimeout.
// A successful dial means a live server owns the socket, so we proceed
// to net.Listen (which surfaces the usual "address already in use"
// error). A failed dial that isStaleSocketErr classifies as stale
// triggers an os.Remove of the path (ignoring fs.ErrNotExist) before
// the bind.
//
// The returned removedStale bool reports whether a stale socket file
// was removed prior to binding so callers can log it. The operation
// is idempotent: removing an absent file is a no-op, and a live
// socket is never removed.
func listen(network, address string) (net.Listener, bool, error) {
	var removedStale bool
	if network == "unix" && address != "" {
		if _, err := os.Stat(address); err == nil {
			conn, dialErr := net.DialTimeout(network, address, staleSocketDialTimeout)
			if dialErr == nil {
				// A live server owns the socket. Fall through to
				// net.Listen so the caller sees the standard
				// "address already in use" error.
				conn.Close()
			} else if isStaleSocketErr(dialErr) {
				rmErr := os.Remove(address)
				switch {
				case rmErr == nil:
					removedStale = true
				case errors.Is(rmErr, fs.ErrNotExist):
					// Another process removed it between our
					// stat and remove; treat as a no-op.
				default:
					return nil, false, rmErr
				}
			}
		}
	}
	//nolint:noctx
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, removedStale, err
	}
	return ln, removedStale, nil
}
