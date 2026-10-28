//go:build darwin || freebsd

package client

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// peerUID returns the uid the server on the other end of raw was running
// as when it started listening.
func peerUID(raw syscall.RawConn, _ string) (uid int, err error) {
	ctrlErr := raw.Control(func(fd uintptr) {
		var cred *unix.Xucred
		cred, err = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err == nil {
			uid = int(cred.Uid)
		}
	})
	if ctrlErr != nil {
		return 0, ctrlErr
	}
	return uid, err
}
