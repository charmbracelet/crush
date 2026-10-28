//go:build !windows

package cmd

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// sighupAfterStopEnv makes TestAwaitStopSIGHUPAfterStop run as its own child
// process.
const sighupAfterStopEnv = "CRUSH_TEST_SIGHUP_AFTER_STOP"

// TestAwaitStopSIGHUPAfterStop checks that a SIGHUP arriving once a stop has
// begun, such as the terminal closing after Ctrl+C, cannot kill the server
// before it has torn down its workspaces and removed its socket. The SIGHUP
// goes to a child process because it would otherwise kill the test binary,
// and signal dispositions are process-wide.
func TestAwaitStopSIGHUPAfterStop(t *testing.T) {
	if os.Getenv(sighupAfterStopEnv) == "1" {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.NoError(t, awaitStop(ctx, nil, make(chan error)))

		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGHUP))
		// Give a fatal SIGHUP time to land before the child exits.
		time.Sleep(200 * time.Millisecond)
		return
	}

	t.Parallel()

	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$")
	child.Env = append(os.Environ(), sighupAfterStopEnv+"=1")
	out, err := child.CombinedOutput()
	require.NoError(t, err, "child output:\n%s", out)
}
