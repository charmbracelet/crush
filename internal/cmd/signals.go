package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
)

// terminationSignals are the signals that mean "stop now, but tidily":
// Ctrl+C, SIGTERM from a service manager (Go also delivers a closed Windows
// console as SIGTERM), and SIGHUP from a closed terminal. The server reads
// SIGHUP as a config reload instead; see awaitStop.
func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}

// notifyContext is signal.NotifyContext, except that the context's cause
// keeps the signal that cancelled it, so signalExitCode can report it.
func notifyContext(parent context.Context, sigs ...os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)
	go func() {
		select {
		case sig := <-ch:
			cancel(signalError{sig})
		case <-ctx.Done():
		}
	}()
	return ctx, func() {
		cancel(nil)
		signal.Stop(ch)
	}
}

// signalExitCode returns the status a shell reports for a process killed by
// the signal that cancelled ctx, 128 plus its number (130 for Ctrl+C, 143
// for SIGTERM), or 0 if no signal cancelled ctx.
func signalExitCode(ctx context.Context) int {
	var se signalError
	if !errors.As(context.Cause(ctx), &se) {
		return 0
	}
	if n, ok := se.sig.(syscall.Signal); ok {
		return 128 + int(n)
	}
	// A signal without a number, such as a Plan 9 note, reads as Ctrl+C.
	return 130
}

// signalError is the cause of a context cancelled by notifyContext. Like
// the cause signal.NotifyContext uses, it matches context.Canceled.
type signalError struct{ sig os.Signal }

func (e signalError) Error() string { return e.sig.String() + " signal received" }

func (e signalError) Is(target error) bool { return target == context.Canceled }
