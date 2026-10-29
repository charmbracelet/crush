package cmd

import (
	"context"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSignalExitCode guards every signal exiting 130 instead of 128+signal.
func TestSignalExitCode(t *testing.T) {
	t.Parallel()

	for sig, want := range map[os.Signal]int{os.Interrupt: 130, syscall.SIGHUP: 129, syscall.SIGTERM: 143} {
		ctx, cancel := context.WithCancelCause(t.Context())
		cancel(signalError{sig})
		require.Equal(t, want, signalExitCode(ctx), "exit status for %v", sig)
		require.ErrorIs(t, context.Cause(ctx), context.Canceled)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.Zero(t, signalExitCode(ctx))
}
