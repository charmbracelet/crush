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

func lockFile(ctx context.Context, f *os.File) (func(), error) {
	h := windows.Handle(f.Fd())
	for {
		ol := new(windows.Overlapped)
		flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
		err := windows.LockFileEx(h, flags, 0, math.MaxUint32, math.MaxUint32, ol)
		if err == nil {
			return func() {
				ol := new(windows.Overlapped)
				_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, math.MaxUint32, math.MaxUint32, ol)
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
	ol := new(windows.Overlapped)
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	if err := windows.LockFileEx(h, flags, 0, math.MaxUint32, math.MaxUint32, ol); err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, ErrContended
		}
		return nil, fmt.Errorf("LockFileEx: %w", err)
	}
	return func() {
		ol := new(windows.Overlapped)
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, math.MaxUint32, math.MaxUint32, ol)
	}, nil
}
