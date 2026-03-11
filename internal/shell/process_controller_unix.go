//go:build unix

package shell

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type processController interface {
	Terminate(killTimeout time.Duration) error
	Close() error
}

type basicProcessController struct {
	cmd *exec.Cmd
}

type processGroupController struct {
	*basicProcessController
}

func newProcessController(cmd *exec.Cmd, detached bool) processController {
	basic := &basicProcessController{cmd: cmd}
	if detached {
		return &processGroupController{basicProcessController: basic}
	}
	return basic
}

func (c *basicProcessController) Terminate(killTimeout time.Duration) error {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	return terminateProcess(c.cmd.Process, killTimeout)
}

func (c *basicProcessController) Close() error {
	return nil
}

func (c *processGroupController) Terminate(killTimeout time.Duration) error {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	pid := c.cmd.Process.Pid
	if pid <= 0 {
		return nil
	}
	return terminateProcessGroup(pid, killTimeout)
}

func terminateProcess(process *os.Process, killTimeout time.Duration) error {
	if process == nil {
		return nil
	}
	if killTimeout <= 0 {
		return process.Kill()
	}
	if err := process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return process.Kill()
	}
	time.Sleep(killTimeout)
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func terminateProcessGroup(pid int, killTimeout time.Duration) error {
	target := -pid
	if killTimeout <= 0 {
		if err := syscall.Kill(target, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
	if err := syscall.Kill(target, syscall.SIGINT); err != nil && !errors.Is(err, syscall.ESRCH) {
		if err := syscall.Kill(target, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
	time.Sleep(killTimeout)
	if err := syscall.Kill(target, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
