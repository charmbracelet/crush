package backend

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestTeardown_LeavesOtherWorkspacesShellsRunning guards against a
// workspace teardown killing every shell in the process. Not parallel: the
// background shell manager is a process global.
func TestTeardown_LeavesOtherWorkspacesShellsRunning(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	b := New(context.Background(), nil, func() {})
	b.SetIdleShutdownDelay(time.Hour)
	t.Cleanup(func() { drainBackend(t, b) })

	mgr := shell.GetBackgroundShellManager()
	cids := []string{uuid.NewString(), uuid.NewString()}
	jobs := make([]*shell.BackgroundShell, 2)
	for i, cid := range cids {
		ws, _, err := b.CreateWorkspace(protoWS(t.TempDir(), t.TempDir(), cid))
		require.NoError(t, err)
		jobs[i], err = mgr.Start(context.Background(), ws.App, ws.Path, nil, "sleep 30", cid, 10)
		require.NoError(t, err)
		t.Cleanup(func() { _ = mgr.Kill(jobs[i].ID) })
	}
	exitsWithin := func(job *shell.BackgroundShell, d time.Duration) bool {
		ctx, cancel := context.WithTimeout(t.Context(), d)
		defer cancel()
		return job.WaitContext(ctx)
	}

	require.NoError(t, b.RetireClient(cids[0]))
	require.True(t, exitsWithin(jobs[0], 5*time.Second), "teardown must kill its own shell")
	require.False(t, exitsWithin(jobs[1], 300*time.Millisecond), "teardown killed another workspace's shell")
}
