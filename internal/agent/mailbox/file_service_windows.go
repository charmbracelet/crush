//go:build windows

package mailbox

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	lockFileEx      = kernel32.NewProc("LockFileEx")
	unlockFile      = kernel32.NewProc("UnlockFile")
)

const (
	LOCKFILE_EXCLUSIVE_LOCK = 0x00000002
	LOCKFILE_FAIL_IMMEDIATELY = 0x00000001
)

// tryLockWindows uses LockFileEx for Windows systems.
func tryLockWindows(f *os.File) error {
	var overlapped syscall.Overlapped
	ret, _, err := lockFileEx.Call(
		uintptr(f.Fd()),
		LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY,
		0,
		0xFFFFFFFF,
		0xFFFFFFFF,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if ret == 0 {
		return fmt.Errorf("LockFileEx: %w", err)
	}
	return nil
}

// unlockWindows releases the lock on Windows systems.
func unlockWindows(f *os.File) error {
	ret, _, err := unlockFile.Call(
		uintptr(f.Fd()),
		0,
		0,
		0xFFFFFFFF,
		0xFFFFFFFF,
	)
	if ret == 0 {
		return fmt.Errorf("UnlockFile: %w", err)
	}
	return nil
}

// tryLockUnix is not used on Windows.
func tryLockUnix(f *os.File) error {
	return fmt.Errorf("not implemented on windows")
}

// unlockUnix is not used on Windows.
func unlockUnix(f *os.File) error {
	return nil
}
