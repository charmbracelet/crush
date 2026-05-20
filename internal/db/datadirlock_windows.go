//go:build windows

package db

import (
	"errors"
	"fmt"
	"math"
	"os"

	"golang.org/x/sys/windows"
)

// errLockContended is returned by tryFileLock when the lock is held
// by another process.
var errLockContended = errors.New("file lock is held by another process")

// tryFileLock takes an exclusive non-blocking lock on path via
// LockFileEx. On success it returns a release function that unlocks
// and closes the descriptor.
//
// The flags combine LOCKFILE_EXCLUSIVE_LOCK with LOCKFILE_FAIL_IMMEDIATELY
// to mirror the BSD LOCK_EX|LOCK_NB semantics used on POSIX. The lock
// is released when the file handle closes, including on process exit,
// which gives us automatic stale-lock recovery without any bookkeeping.
func tryFileLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	h := windows.Handle(f.Fd())
	ol := new(windows.Overlapped)
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	if err := windows.LockFileEx(h, flags, 0, math.MaxUint32, math.MaxUint32, ol); err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, errLockContended
		}
		return nil, fmt.Errorf("LockFileEx: %w", err)
	}
	return func() {
		ol := new(windows.Overlapped)
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, math.MaxUint32, math.MaxUint32, ol)
		_ = f.Close()
	}, nil
}
