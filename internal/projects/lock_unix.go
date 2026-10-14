//go:build !windows

package projects

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive inter-process lock on path and returns the
// function that releases it.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	// Closing the descriptor releases the flock.
	return func() { f.Close() }, nil
}
