//go:build windows
// +build windows

package cmd

import (
	"errors"
	"log/slog"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func detachProcess(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS
}

// startDetached starts the command newCmd builds, detached from this
// process and outside the job object this process is in, if any. A child
// stays in its parent's job unless it breaks away, and some jobs kill their
// processes when they close: Win32-OpenSSH runs each session in one, which
// would take a server shared with other terminals down with the session
// that spawned it. A job that does not allow breakaway makes the start fail
// with ERROR_ACCESS_DENIED, and the command is then started inside the job
// from a new exec.Cmd, since an exec.Cmd cannot be started twice.
func startDetached(newCmd func() *exec.Cmd) (*exec.Cmd, error) {
	c := newCmd()
	detachProcess(c)
	c.SysProcAttr.CreationFlags |= windows.CREATE_BREAKAWAY_FROM_JOB
	err := c.Start()
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		slog.Warn("Failed to start process outside the job object, retrying inside it", "error", err)
		c = newCmd()
		detachProcess(c)
		err = c.Start()
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}
