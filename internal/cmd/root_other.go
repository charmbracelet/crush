//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

func detachProcess(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setsid = true
}

// releaseProcess lets go of a started detached process. Setsid does not
// reparent it, so this process stays its parent until one of them exits:
// waiting in the background reaps it if it exits first, instead of leaving
// a zombie behind until this process exits.
func releaseProcess(c *exec.Cmd) error {
	go func() { _ = c.Wait() }()
	return nil
}
