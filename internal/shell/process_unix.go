//go:build unix

package shell

import (
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// killTimeout is how long to wait after sending SIGINT before sending
// SIGKILL when cancelling an external command. Matches mvdan.cc/sh's
// DefaultExecHandler default so users see consistent grace periods
// regardless of which path executed their command.
const killTimeout = 2 * time.Second

// prepareCmd configures cmd to run in its own process group so that on
// cancellation we can signal the entire descendant tree, not just the
// direct child. Without this, a child that fork+execs an editor (git
// spawning vim, npm spawning a dev server, etc.) leaves the grandchild
// reparented to PID 1 when we kill the child.
func prepareCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// interruptCmd sends SIGINT to the entire process group. The negative
// PID is the kernel's way of saying "this PGID, not this PID". Safe to
// call after Start; a no-op if the process is already reaped.
func interruptCmd(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return unix.Kill(-cmd.Process.Pid, unix.SIGINT)
}

// killCmd sends SIGKILL to the entire process group. Same negative-PID
// convention as interruptCmd.
func killCmd(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return unix.Kill(-cmd.Process.Pid, unix.SIGKILL)
}
