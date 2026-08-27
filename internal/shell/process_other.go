//go:build !unix

package shell

import (
	"os/exec"
	"time"
)

// killTimeout is how long to wait after sending interrupt before sending kill.
// On Windows, we send kill immediately since Go doesn't support Interrupt.
const killTimeout = 0 * time.Second

// prepareCmd is a no-op on non-Unix platforms.
func prepareCmd(cmd *exec.Cmd) {}

// interruptCmd kills the process on non-Unix platforms (no SIGINT support).
func interruptCmd(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// killCmd kills the process.
func killCmd(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
