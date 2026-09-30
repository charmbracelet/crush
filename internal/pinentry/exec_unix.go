//go:build !windows

package pinentry

import (
	"context"
	"os/exec"
	"syscall"
)

// prepareProcess isolates the child from Crush's controlling terminal and
// session so a shell spawned within it cannot take over the TTY or signal
// Crush's process group.
func prepareProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}

// watchCancel kills the child's process group when ctx is done. The
// returned function must be called after [exec.Cmd.Wait] and is safe to
// call any number of times.
func watchCancel(ctx context.Context, cmd *exec.Cmd) (stop func()) {
	stopf := context.AfterFunc(ctx, func() {
		if cmd.Process == nil {
			return
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	})
	return func() { stopf() }
}
