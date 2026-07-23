//go:build unix

package shell

import (
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// killTimeout is how long to wait after sending SIGINT before sending SIGKILL.
const killTimeout = 2 * time.Second

// prepareCmd sets up process group isolation for the command so we can
// kill the entire process tree on cancellation.
func prepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// interruptCmd sends SIGINT to the process group.
func interruptCmd(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return unix.Kill(-cmd.Process.Pid, unix.SIGINT)
}

// killCmd sends SIGKILL to the process group.
func killCmd(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return unix.Kill(-cmd.Process.Pid, unix.SIGKILL)
}
