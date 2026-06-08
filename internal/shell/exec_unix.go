//go:build !windows

package shell

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"mvdan.cc/sh/v3/interp"
)

// defaultKillTimeout matches mvdan's DefaultExecHandler default. Extracted
// so the coupling to upstream is explicit rather than a buried literal.
const defaultKillTimeout = 2 * time.Second

// isolateProcess sets SysProcAttr so the child runs in its own session,
// fully detached from Crush's controlling terminal. This prevents shells
// like zsh from grabbing the TTY for job control, which would cause
// "suspended (tty input)" and corrupt Bubble Tea's terminal rendering.
// It also prevents child processes from sending signals to Crush's process
// group.
func isolateProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}

// processGroupExecHandler returns an ExecHandlerFunc that replaces
// interp.DefaultExecHandler with one that fully isolates child processes
// from Crush's session and controlling terminal.
//
// Without this, shells like zsh that enable job control when sourcing
// framework files will attempt to take over the TTY, causing SIGTTIN/SIGTTOU
// signals and corrupting the parent terminal state.
//
// The implementation mirrors interp.DefaultExecHandler with two additions:
// isolateProcess(&cmd) after construction, and negative-PID signal targeting
// in the cancellation callback so the entire child process group is killed.
func processGroupExecHandler(killTimeout time.Duration) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		hc := interp.HandlerCtx(ctx)
		path, err := interp.LookPathDir(hc.Dir, hc.Env, args[0])
		if err != nil {
			fmt.Fprintln(hc.Stderr, err)
			return interp.ExitStatus(127)
		}

		cmd := exec.Cmd{
			Path:   path,
			Args:   args,
			Env:    execEnvList(hc.Env),
			Dir:    hc.Dir,
			Stdin:  hc.Stdin,
			Stdout: hc.Stdout,
			Stderr: hc.Stderr,
		}
		isolateProcess(&cmd)

		err = cmd.Start()
		if err == nil {
			stopf := context.AfterFunc(ctx, func() {
				if killTimeout <= 0 {
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
					return
				}
				// Signal the child's process group (negative PID) so
				// grandchildren also receive it.
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
				time.Sleep(killTimeout)
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			})
			defer stopf()

			err = cmd.Wait()
		}

		return exitStatusFromError(ctx, hc.Stderr, err)
	}
}

// exitStatusFromError translates an exec error into an interp exit status,
// matching the conventions of interp.DefaultExecHandler. Extracted so the
// platform-specific exec handler stays focused on isolation mechanics.
func exitStatusFromError(ctx context.Context, stderr io.Writer, err error) error {
	if err == nil {
		return nil
	}
	switch err := err.(type) {
	case *exec.ExitError:
		if status, ok := err.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return interp.ExitStatus(128 + uint8(status.Signal()))
		}
		return interp.ExitStatus(uint8(err.ExitCode()))
	case *exec.Error:
		fmt.Fprintf(stderr, "%v\n", err)
		return interp.ExitStatus(127)
	default:
		return err
	}
}
