//go:build windows
// +build windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func init() {
	// Set console code pages to UTF-8 to prevent garbled Chinese output on
	// Windows systems with GBK locale (cp936). See #3656 and #839.
	// Errors are ignored when not attached to a console (piped or detached).
	_ = windows.SetConsoleCP(65001)
	_ = windows.SetConsoleOutputCP(65001)

	// Ensure child processes use UTF-8 even when stdout is piped, where the
	// console code page is ignored. This covers python -c print cases.
	_ = os.Setenv("PYTHONIOENCODING", "utf-8")
	_ = os.Setenv("PYTHONUTF8", "1")
}

func detachProcess(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS
}
