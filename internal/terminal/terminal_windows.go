//go:build windows

package terminal

import "syscall"

// sysProcAttr returns the syscall.SysProcAttr for Windows systems.
// On Windows, ConPTY handles process isolation differently, so we don't
// need special attributes.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
