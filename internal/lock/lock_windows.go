//go:build windows

package lock

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// retrySleep is the interval between non-blocking lock retries in the
// blocking File path.
const retrySleep = 100 * time.Millisecond

// lockOffset is the offset of the single byte every acquirer locks. It
// must not change, as acquirers that lock different bytes do not exclude
// each other. Byte-range locks on Windows are mandatory: a locked range
// cannot be read or written through any other handle, even one opened by
// the locking process. Locking the whole file, as older releases do,
// would stop a holder recording details such as its pid in the file and
// stop contenders reading them. The byte lies far beyond the end of any
// lock file, so the contents stay accessible, and inside the range older
// releases lock, so they and newer releases still exclude each other.
const lockOffset uint64 = math.MaxInt64 - 1

// lockOverlapped returns a fresh OVERLAPPED that addresses lockOffset.
func lockOverlapped() *windows.Overlapped {
	return &windows.Overlapped{
		Offset:     uint32(lockOffset & math.MaxUint32),
		OffsetHigh: uint32(lockOffset >> 32),
	}
}

func lockFile(ctx context.Context, f *os.File) (func(), error) {
	h := windows.Handle(f.Fd())
	for {
		ol := lockOverlapped()
		flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
		err := windows.LockFileEx(h, flags, 0, 1, 0, ol)
		if err == nil {
			return func() {
				ol := lockOverlapped()
				_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
			}, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) && !errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, fmt.Errorf("LockFileEx: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("acquire lock: %w", ctx.Err())
		case <-time.After(retrySleep):
		}
	}
}

func tryLockFile(f *os.File) (func(), error) {
	h := windows.Handle(f.Fd())
	ol := lockOverlapped()
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	if err := windows.LockFileEx(h, flags, 0, 1, 0, ol); err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, ErrContended
		}
		return nil, fmt.Errorf("LockFileEx: %w", err)
	}
	return func() {
		ol := lockOverlapped()
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
	}, nil
}
