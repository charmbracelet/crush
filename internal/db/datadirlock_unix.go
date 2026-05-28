//go:build !windows

package db

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// errLockContended is returned by tryFileLock when the lock is already
// held by another open file description (typically another process).
var errLockContended = errors.New("file lock is held by another process")

// tryFileLock takes an exclusive non-blocking BSD flock on path,
// creating the file if necessary. On success it returns a release
// function that drops the lock and closes the descriptor. When the
// lock is contended it returns errLockContended.
//
// BSD flock is advisory and per-open-file-description, so it does not
// interfere with the byte-range locks SQLite itself uses on the same
// file's siblings (crush.db, crush.db-wal, crush.db-shm). The lock is
// also released automatically by the kernel when the file descriptor
// is closed, including on process crash, so we do not need any
// explicit stale-lock recovery.
func tryFileLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, errLockContended
		}
		return nil, fmt.Errorf("flock: %w", err)
	}
	return func() {
		// Closing the descriptor releases the flock atomically.
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
