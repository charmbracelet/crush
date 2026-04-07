//go:build !windows

package mailbox

import (
	"fmt"
	"os"
	"syscall"
)

// tryLockUnix uses flock for Unix systems.
func tryLockUnix(f *os.File) error {
	// LOCK_EX | LOCK_NB for non-blocking exclusive lock.
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		return fmt.Errorf("flock: %w", err)
	}
	return nil
}

// unlockUnix releases flock on Unix systems.
func unlockUnix(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// tryLockWindows is not used on Unix.
func tryLockWindows(f *os.File) error {
	return fmt.Errorf("not implemented on unix")
}

// unlockWindows is not used on Unix.
func unlockWindows(f *os.File) error {
	return nil
}
