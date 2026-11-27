//go:build windows

package shell

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"mvdan.cc/sh/v3/interp"
)

// processGroupExecHandler returns the Windows counterpart of the Unix
// handler. Windows has no process groups to signal, so cancellation kills
// the process directly. Unlike interp.DefaultExecHandler on Windows, it
// sets a WaitDelay, so a descendant still holding a stdio pipe open (a Git
// Bash script's background child, say) cannot block Wait forever.
func processGroupExecHandler(killTimeout time.Duration) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		hc := interp.HandlerCtx(ctx)
		path, err := interp.LookPathDir(hc.Dir, hc.Env, args[0])
		if err != nil {
			fmt.Fprintln(hc.Stderr, err)
			return interp.ExitStatus(127)
		}

		cmd := exec.CommandContext(ctx, path)
		cmd.Args = args
		cmd.Env = execEnvList(hc.Env)
		cmd.Dir = hc.Dir
		cmd.Stdin = hc.Stdin
		cmd.Stdout = hc.Stdout
		cmd.Stderr = hc.Stderr
		cmd.Cancel = func() error { return cmd.Process.Kill() }
		// Bound how long Wait blocks after the process exits or is killed.
		cmd.WaitDelay = killTimeout + time.Second

		err = cmd.Run()
		// A killed process exits with a plain code on Windows, so report
		// the cancellation itself rather than that code.
		if err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		return exitStatusFromError(ctx, hc.Stderr, err)
	}
}
