//go:build !windows

package client

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

// geteuid returns the uid the server must run as: our effective uid, which
// is what peer credentials and file owners record. Tests replace it to
// pretend the server belongs to another user.
var geteuid = os.Geteuid

// verifyUnixPeer fails unless the server on the other end of conn, a
// connection to the unix socket at path, runs as the current user.
func verifyUnixPeer(conn net.Conn, path string) error {
	uid, err := connPeerUID(conn, path)
	if err != nil {
		return fmt.Errorf("failed to verify the owner of server socket %q: %w", path, err)
	}
	if want := geteuid(); uid != want {
		return fmt.Errorf("server socket %q is owned by another user (uid %d, not %d)", path, uid, want)
	}
	return nil
}

// connPeerUID returns the uid of the server on the other end of conn.
func connPeerUID(conn net.Conn, path string) (int, error) {
	sc, ok := conn.(syscall.Conn)
	if !ok {
		return 0, fmt.Errorf("unsupported connection type %T", conn)
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return 0, err
	}
	return peerUID(raw, path)
}
