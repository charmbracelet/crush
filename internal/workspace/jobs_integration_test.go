package workspace_test

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// dataDir returns a scratch directory for the workspace database whose
// removal is best-effort.
//
// t.TempDir() cannot be used: this harness deliberately leaves the created
// workspace running for the lifetime of the process (there is no teardown
// hook that closes the backend's app), so its sqlite handle on crush.db is
// still open at cleanup time. POSIX unlinks an open file happily; Windows
// refuses, and t.TempDir() turns that refusal into a test failure —
// "TempDir RemoveAll cleanup: ...\\crush.db: The process cannot access the
// file because it is being used by another process" on windows-latest.
func dataDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "crush-jobs-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// startJobs registers n real background shells in the process-global
// registry and removes them again afterwards. In production that registry
// lives in the *server* process; here client and server share a process, so
// the test can seed the server side directly and then read it back over
// HTTP exactly as a remote TUI would.
func startJobs(t *testing.T, n int) []string {
	t.Helper()

	manager := shell.GetBackgroundShellManager()
	ids := make([]string, 0, n)
	for range n {
		bs, err := manager.Start(context.Background(), t.TempDir(), nil, "sleep 3000", "job")
		require.NoError(t, err)
		ids = append(ids, bs.ID)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_ = manager.Kill(id)
			_ = manager.Remove(id)
		}
	})
	return ids
}

// pick returns the jobs from got whose IDs are in want, preserving got's
// order. The registry is process-global, so a sibling test could have left
// unrelated jobs behind; asserting on the subset keeps this hermetic while
// still pinning relative order.
func pick(got []proto.BackgroundJob, want []string) []string {
	var out []string
	for _, j := range got {
		if slices.Contains(want, j.ID) {
			out = append(out, j.ID)
		}
	}
	return out
}

// TestClientWorkspace_ListAndKillBackgroundJobsOverHTTP is the
// CRUSH_CLIENT_SERVER=1 proof for the jobs dialog's read and kill paths.
//
// The dialog used to call shell.GetBackgroundShellManager() straight from
// the TUI. In client/server mode the TUI is a different process from the
// agent, so that registry is permanently empty there: /jobs showed nothing
// and Ctrl+B-created jobs were invisible and unkillable. Both calls now go
// through Workspace like the Ctrl+B release already did, so this drives the
// real stack — ClientWorkspace -> client.Client -> HTTP -> server ->
// backend -> the registry — and not an in-process shortcut.
func TestClientWorkspace_ListAndKillBackgroundJobsOverHTTP(t *testing.T) {
	xdgIsolate(t)
	rt := newRuntimeServer(t)

	cwd := t.TempDir()
	c := rt.newClient(t, cwd)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	wsProto, err := c.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir: dataDir(t)})
	require.NoError(t, err)
	ws := workspace.NewClientWorkspace(c, *wsProto)

	ids := startJobs(t, 3)

	jobs, err := ws.ListBackgroundJobs(ctx)
	require.NoError(t, err)
	require.Equal(t, ids, pick(jobs, ids),
		"a client-mode TUI must see the agent process's jobs, oldest first")

	// The listing must carry enough to render a row, and nothing more:
	// see proto.BackgroundJob on why output is deliberately absent.
	var seeded *proto.BackgroundJob
	for i := range jobs {
		if jobs[i].ID == ids[0] {
			seeded = &jobs[i]
		}
	}
	require.NotNil(t, seeded)
	require.Equal(t, "sleep 3000", seeded.Command)
	require.Equal(t, "job", seeded.Description)
	require.False(t, seeded.StartedAt.IsZero(), "the row renders elapsed time from StartedAt")
	require.False(t, seeded.Done)

	// Killing must reach the shell in the agent process.
	require.NoError(t, ws.KillBackgroundJob(ctx, ids[1]))

	after, err := ws.ListBackgroundJobs(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{ids[0], ids[2]}, pick(after, ids),
		"the killed job must be gone from the server's registry")
	require.NotContains(t, shell.GetBackgroundShellManager().List(), ids[1],
		"kill must terminate the real shell, not just hide a row")
}

// TestClientWorkspace_KillUnknownBackgroundJobIs404 pins the error shape of
// the kill path. A job that finished and was cleaned up between the client
// listing it and confirming the kill is an expected race; it must surface
// as a 404 the UI can report, not a 500 and not a silent success.
func TestClientWorkspace_KillUnknownBackgroundJobIs404(t *testing.T) {
	xdgIsolate(t)
	rt := newRuntimeServer(t)

	cwd := t.TempDir()
	c := rt.newClient(t, cwd)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	wsProto, err := c.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir: dataDir(t)})
	require.NoError(t, err)
	ws := workspace.NewClientWorkspace(c, *wsProto)

	err = ws.KillBackgroundJob(ctx, "no-such-job")
	require.Error(t, err, "killing a job that is gone must not report success")
	require.Contains(t, err.Error(), "status code 404")
}
