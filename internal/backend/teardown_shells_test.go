package backend

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestTeardown_LeavesOtherWorkspacesShellsRunning is the regression test for
// a workspace teardown that killed every shell in the server process:
// App.Shutdown ran the process-wide KillAll, so a client leaving workspace A
// also killed the commands and background jobs workspace B's agent was
// running. Each teardown must stop only its own app's shells. Not parallel:
// the background shell manager is a process global.
func TestTeardown_LeavesOtherWorkspacesShellsRunning(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")

	b := New(context.Background(), nil, func() {})
	b.SetIdleShutdownDelay(time.Hour)
	t.Cleanup(func() { drainBackend(t, b) })

	cidA, cidB := uuid.NewString(), uuid.NewString()
	wsA, _, err := b.CreateWorkspace(protoWS(t.TempDir(), t.TempDir(), cidA))
	require.NoError(t, err)
	wsB, _, err := b.CreateWorkspace(protoWS(t.TempDir(), t.TempDir(), cidB))
	require.NoError(t, err)

	// Start a long command in each workspace, tagged with its app the way
	// that workspace's bash tool tags it.
	mgr := shell.GetBackgroundShellManager()
	jobA, err := mgr.Start(context.Background(), wsA.App, wsA.Path, nil, "sleep 30", "A", 10)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.Kill(jobA.ID) })
	jobB, err := mgr.Start(context.Background(), wsB.App, wsB.Path, nil, "sleep 30", "B", 10)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.Kill(jobB.ID) })

	exitsWithin := func(job *shell.BackgroundShell, d time.Duration) bool {
		ctx, cancel := context.WithTimeout(t.Context(), d)
		defer cancel()
		return job.WaitContext(ctx)
	}

	// A's only client exits, which tears A down before RetireClient returns.
	require.NoError(t, b.RetireClient(cidA))
	require.True(t, exitsWithin(jobA, 5*time.Second), "workspace A's teardown must kill A's own shell")
	require.False(t, exitsWithin(jobB, 300*time.Millisecond), "workspace A's teardown killed workspace B's running shell")

	require.NoError(t, b.RetireClient(cidB))
	require.True(t, exitsWithin(jobB, 5*time.Second), "workspace B's teardown must kill B's own shell")
}
