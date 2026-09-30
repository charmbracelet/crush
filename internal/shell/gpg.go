package shell

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/crush/internal/pinentry"
	"mvdan.cc/sh/v3/interp"
)

// gpgHandler intercepts top-level gpg commands run from Crush's shell
// and drives them through the integrated pinentry runner, so a
// passphrase/PIN prompt is rendered by the TUI instead of an external
// terminal pinentry. Commands that are not credential-relevant, or that
// fail loopback pinentry (older GPG, agent without loopback support,
// headless run), fall back to the next handler unchanged, which reverts
// to the existing terminal-handover behavior.
func gpgHandler() execMiddleware {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if !pinentry.ShouldIntercept(args) {
				return next(ctx, args)
			}
			runner := pinentry.DefaultRunner()
			if runner == nil {
				return next(ctx, args)
			}

			hc := interp.HandlerCtx(ctx)
			err := runner.Run(ctx, pinentry.RunOptions{
				Args:   args,
				Env:    execEnvList(hc.Env),
				Dir:    hc.Dir,
				Stdin:  hc.Stdin,
				Stdout: hc.Stdout,
				Stderr: hc.Stderr,
			})
			if errors.Is(err, pinentry.ErrFallback) {
				return next(ctx, args)
			}
			if err == nil {
				return nil
			}
			// Map the runner's errors back to interpreter exit
			// statuses so the shell behaves as gpg would have.
			var ee *pinentry.ExitError
			switch {
			case errors.Is(err, pinentry.ErrCancelled):
				// Pinentry cancellation is GPG's exit code 2.
				return interp.ExitStatus(2)
			case errors.As(err, &ee):
				return interp.ExitStatus(uint8(ee.Code))
			case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
				return err
			default:
				fmt.Fprintln(hc.Stderr, err)
				return interp.ExitStatus(1)
			}
		}
	}
}
