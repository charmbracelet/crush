//go:build unix

package terminal

import "syscall"

// sysProcAttr returns the syscall.SysProcAttr for Unix systems.
// It creates a new session and sets the controlling terminal to isolate
// the subprocess from the parent terminal.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}
}
