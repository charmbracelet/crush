//go:build unix

package terminal

import (
	"os/exec"
	"syscall"
)

// sysProcAttr returns the syscall.SysProcAttr for Unix systems.
// It creates a new session and sets the controlling terminal to isolate
// the subprocess from the parent terminal.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}
}

// killProcessGroup sends SIGTERM to the entire process group of cmd.
// Since Setsid is true, the child is its own session leader and all
// descendants share its process group.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// Negative PID signals the whole process group.
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

// forceKillProcessGroup sends SIGKILL to the entire process group.
func forceKillProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
