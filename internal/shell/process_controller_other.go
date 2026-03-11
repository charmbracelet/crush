//go:build !windows && !unix

package shell

import (
	"os/exec"
	"time"
)

type processController interface {
	Terminate(killTimeout time.Duration) error
	Close() error
}

type basicProcessController struct {
	cmd *exec.Cmd
}

func newProcessController(cmd *exec.Cmd, detached bool) processController {
	return &basicProcessController{cmd: cmd}
}

func (c *basicProcessController) Terminate(_ time.Duration) error {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	return c.cmd.Process.Kill()
}

func (c *basicProcessController) Close() error {
	return nil
}
