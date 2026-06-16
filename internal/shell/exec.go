package shell

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"mvdan.cc/sh/v3/interp"
)

// processGroupExecHandler is the terminal middleware in the Crush exec
// handler chain. It owns the final hop into os/exec for every external
// command — bare or path-prefixed — so that on context cancellation we
// can signal the entire process tree, not just the direct child.
//
// This intentionally replaces mvdan.cc/sh's [interp.DefaultExecHandler],
// which uses exec.CommandContext + Process.Kill on the direct child only.
// Without this replacement, commands like `git rebase --continue` that
// fork an editor leak the editor when the tool call is cancelled. The
// middleware is the last entry in [standardHandlers] and never invokes
// next; mvdan's runtime auto-appends DefaultExecHandler after the user
// chain, and that fallback is exactly what we are trying to displace.
//
// PATH resolution mirrors [interp.LookPathDir] (the same primitive
// DefaultExecHandler uses), so users see identical resolution semantics
// regardless of which handler runs the binary.
func processGroupExecHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}
			hc := interp.HandlerCtx(ctx)
			path, err := interp.LookPathDir(hc.Dir, hc.Env, args[0])
			if err != nil {
				fmt.Fprintln(hc.Stderr, err)
				return interp.ExitStatus(127)
			}
			return runExternal(ctx, path, args)
		}
	}
}

// runExternal launches path with the given argv under our process-group
// cancellation regime. argv[0] is preserved as the visible program name
// (matching how mvdan.cc/sh builds exec.Cmd); path is the already-resolved
// absolute binary.
//
// Cancellation flow on Unix:
//  1. context.AfterFunc fires when ctx is cancelled.
//  2. We send SIGINT to the process group (-pid).
//  3. After [killTimeout], we send SIGKILL to the process group.
//
// The two-stage signal lets well-behaved children clean up (close PTYs,
// flush buffers) before the kernel mows them down. killTimeout is 0 on
// platforms without SIGINT support; the closure collapses to a single
// kill in that case.
//
// On normal completion, defer stops the AfterFunc so we never signal a
// reaped pid.
func runExternal(ctx context.Context, path string, args []string) error {
	hc := interp.HandlerCtx(ctx)
	cmd := exec.Cmd{
		Path:   path,
		Args:   args,
		Env:    execEnvList(hc.Env),
		Dir:    hc.Dir,
		Stdin:  hc.Stdin,
		Stdout: hc.Stdout,
		Stderr: hc.Stderr,
	}
	prepareCmd(&cmd)

	if err := cmd.Start(); err != nil {
		// LookPathDir already validated existence, so a Start failure
		// here is almost always a permission or fork error. Surface it
		// the same way mvdan's DefaultExecHandler does — message on
		// stderr, 127 exit code — to keep UX consistent with the case
		// where LookPathDir itself fails.
		fmt.Fprintln(hc.Stderr, err)
		return interp.ExitStatus(127)
	}

	stopCancel := context.AfterFunc(ctx, func() {
		if killTimeout <= 0 {
			_ = killCmd(&cmd)
			return
		}
		_ = interruptCmd(&cmd)
		time.Sleep(killTimeout)
		_ = killCmd(&cmd)
	})
	defer stopCancel()

	waitErr := cmd.Wait()
	return translateExitError(ctx, waitErr)
}

// translateExitError maps an exec.Cmd.Wait error into the form mvdan's
// interpreter expects:
//
//   - nil           → nil
//   - cancellation  → ctx.Err() (so callers can errors.Is for Canceled /
//     DeadlineExceeded; matches IsInterrupt's contract)
//   - non-zero exit → interp.ExitStatus(code), with signaled exits
//     reported as 128 since we no longer have signal-specific info on
//     all platforms
//   - other errors  → returned verbatim so the runner aborts
//
// The ctx.Err() check has to come first: when our AfterFunc kills the
// process, Wait returns an ExitError whose ExitCode is -1 (signaled). We
// want callers to see Canceled rather than a synthetic exit code, since
// the synthetic code can't be distinguished from a genuine SIGINT the
// user typed.
func translateExitError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		if code < 0 {
			// Signaled but ctx wasn't cancelled — e.g. the user sent
			// SIGTERM from outside. Report a non-zero status so the
			// interpreter sees failure, without claiming a specific
			// signal number we no longer track.
			code = 128
		}
		return interp.ExitStatus(uint8(code))
	}
	return err
}
