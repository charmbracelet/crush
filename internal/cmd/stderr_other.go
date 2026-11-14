//go:build !windows

package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"golang.org/x/sys/unix"
)

// detachStderr moves fd 2 to /dev/null so forked children cannot print over
// the TUI between fork and exec, while Go's stderr keeps the terminal. The
// returned restore func is always safe to call.
func detachStderr() (restore func(), err error) {
	noop := func() {}

	saved, err := unix.FcntlInt(uintptr(unix.Stderr), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return noop, fmt.Errorf("duplicate stderr: %w", err)
	}
	terminal := os.NewFile(uintptr(saved), "/dev/stderr")

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		err = unix.Dup2(int(devNull.Fd()), unix.Stderr)
		devNull.Close()
	}
	if err != nil {
		terminal.Close()
		return noop, fmt.Errorf("redirect stderr: %w", err)
	}

	original := os.Stderr
	os.Stderr = terminal
	_ = debug.SetCrashOutput(terminal, debug.CrashOptions{})

	return func() {
		_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
		_ = unix.Dup2(saved, unix.Stderr)
		os.Stderr = original
		terminal.Close()
	}, nil
}
