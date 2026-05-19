//go:build windows

package cmd

import (
	"fmt"
	"math"
	"os"

	"golang.org/x/sys/windows"
)

// acquireSpawnLock takes an exclusive lock on the given file (creating
// it if necessary) using LockFileEx, and returns a release function
// that unlocks and closes the file. Blocks until the lock is acquired.
func acquireSpawnLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open spawn lock %q: %v", path, err)
	}
	h := windows.Handle(f.Fd())
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, math.MaxUint32, math.MaxUint32, ol); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("LockFileEx spawn lock %q: %v", path, err)
	}
	return func() {
		ol := new(windows.Overlapped)
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, math.MaxUint32, math.MaxUint32, ol)
		_ = f.Close()
	}, nil
}
