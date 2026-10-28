//go:build !windows && !linux && !darwin && !freebsd

package client

import (
	"fmt"
	"os"
	"syscall"
)

// peerUID returns the owner of the socket file at path. Peer credentials
// are only looked up on Linux, macOS and FreeBSD; elsewhere the owner
// stands in for them, since in a sticky directory such as /tmp only that
// user can replace the file.
func peerUID(_ syscall.RawConn, path string) (int, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("unsupported file info %T", fi.Sys())
	}
	return int(st.Uid), nil
}
