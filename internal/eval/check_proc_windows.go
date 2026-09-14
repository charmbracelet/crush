//go:build windows

package eval

import "os/exec"

// checkProcAttr — Windows kills the process tree via Job objects in
// os/exec already; nothing extra needed here.
func checkProcAttr(c *exec.Cmd) {}

// killCheckGroup falls back to the process kill — Windows process
// groups aren't signaled by pid.
func killCheckGroup(c *exec.Cmd) error {
	if c.Process == nil {
		return nil
	}
	return c.Process.Kill()
}
