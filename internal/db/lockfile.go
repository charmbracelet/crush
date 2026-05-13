//go:build unix

package db

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// tryExclusiveLock attempts to acquire an exclusive advisory lock on
// the given file path using flock. Returns the locked file handle on
// success. If another process holds the lock, returns a descriptive
// error. The lock is automatically released when the file is closed
// or the process exits.
func tryExclusiveLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf(
			"another Crush instance is using this project (exclusive locking mode). " +
				"Close it first or disable sqlite_exclusive_lock in crush.json",
		)
	}

	return f, nil
}
