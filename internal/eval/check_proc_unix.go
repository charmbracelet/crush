//go:build !windows

package eval

import (
	"os/exec"
	"syscall"
)

// checkProcAttr puts the check in its own process group so the kill
// reaches children the script spawned.
func checkProcAttr(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setpgid = true
}

// killCheckGroup SIGKILLs the check's whole process group.
func killCheckGroup(c *exec.Cmd) error {
	if c.Process == nil {
		return nil
	}
	return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
}
