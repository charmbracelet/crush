//go:build !windows

package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/lock"
)

// staleSocketDialTimeout bounds the probe used to detect whether a Unix
// socket file on disk is backed by a live listener.
const staleSocketDialTimeout = 200 * time.Millisecond

// socketLockTimeout bounds the wait for another process that is binding
// the same Unix socket path. It holds the lock for one probe and one
// bind, so the wait is normally a fraction of a second.
const socketLockTimeout = 5 * time.Second

// dialSocket probes the socket file at address for a live listener. A
// package variable so tests can stall a caller between its probe and the
// removal that follows.
var dialSocket = func(network, address string) (net.Conn, error) {
	return net.DialTimeout(network, address, staleSocketDialTimeout)
}

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
// The probe, the removal and the bind run under an exclusive lock on
// address+".lock", so servers starting at once on the same path take
// turns. Otherwise one could remove the socket another has just bound,
// leaving that server listening on a deleted file no client can reach.
// The lock file is never removed, because the lock is keyed by inode
// rather than path.
//
// The returned removedStale bool reports whether a stale socket file
// was removed prior to binding so callers can log it. The operation
// is idempotent: removing an absent file is a no-op, and a live
// socket is never removed.
func listen(network, address string) (net.Listener, bool, error) {
	var removedStale bool
	if network == "unix" && address != "" {
		ctx, cancel := context.WithTimeout(context.Background(), socketLockTimeout)
		defer cancel()
		release, err := lock.File(ctx, address+".lock")
		if err != nil {
			return nil, false, fmt.Errorf("lock socket path: %w", err)
		}
		defer release()

		if _, err := os.Stat(address); err == nil {
			conn, dialErr := dialSocket(network, address)
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
	// The socket is created with the process umask, which may let other
	// users connect when it lives in a shared directory such as /tmp.
	// Restrict it to its owner. Linux abstract sockets (a leading '@')
	// have no file to chmod.
	if network == "unix" && !strings.HasPrefix(address, "@") {
		if err := os.Chmod(address, 0o600); err != nil {
			ln.Close()
			return nil, removedStale, err
		}
	}
	return ln, removedStale, nil
}
