//go:build windows

package watcher

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazyDLL("kernel32.dll")
	procGetProcessHandleCount = kernel32.NewProc("GetProcessHandleCount")
)

func Ulimit() (uint64, error) {
	// Windows doesn't have the same file descriptor limits as Unix systems
	// Instead, we can get the current handle count for monitoring purposes
	currentProcess := windows.CurrentProcess()

	var handleCount uint32
	ret, _, err := procGetProcessHandleCount.Call(
		uintptr(currentProcess),
		uintptr(unsafe.Pointer(&handleCount)),
	)

	if ret == 0 {
		// If the call failed, return a reasonable default
		if err != syscall.Errno(0) {
			return 2048, nil
		}
	}

	// Windows typically allows much higher handle counts than Unix file descriptors
	// Return the current count, which serves as a baseline for monitoring
	return uint64(handleCount), nil
}
