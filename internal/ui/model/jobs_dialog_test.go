package model

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// jobsWorkspace is a workspace stub for the background-jobs dialog paths.
// It records what the UI asked for so the tests can pin that the dialog
// reads and kills through the workspace rather than through the
// process-global shell registry, which is empty in the TUI process under
// CRUSH_CLIENT_SERVER=1.
type jobsWorkspace struct {
	countingWorkspace

	jobs    []proto.BackgroundJob
	listErr error

	listCalls int
	killed    []string
	killErr   error
}

func (w *jobsWorkspace) ListBackgroundJobs(context.Context) ([]proto.BackgroundJob, error) {
	w.listCalls++
	return w.jobs, w.listErr
}

func (w *jobsWorkspace) KillBackgroundJob(_ context.Context, jobID string) error {
	w.killed = append(w.killed, jobID)
	return w.killErr
}

func stubJobs() []proto.BackgroundJob {
	base := time.Now().Add(-time.Hour)
	return []proto.BackgroundJob{
		{ID: "srv-a", Command: "server-side-job-a", StartedAt: base},
		{ID: "srv-b", Command: "server-side-job-b", StartedAt: base.Add(time.Second)},
	}
}

// openJobs runs openJobsDialog the way the runtime would: the returned
// command executes off the Update goroutine, and its message is fed back
// through Update.
func openJobs(t *testing.T, m *UI) {
	t.Helper()
	cmd := m.openJobsDialog()
	require.NotNil(t, cmd, "opening /jobs must return a command to fetch off-thread")
	msg := cmd()
	require.IsType(t, jobsLoadedMsg{}, msg)
	m.Update(msg)
}

func openJobsDialogModel(t *testing.T, m *UI) *dialog.Jobs {
	t.Helper()
	d := m.dialog.Dialog(dialog.JobsID)
	require.NotNil(t, d, "the jobs dialog must be open")
	j, ok := d.(*dialog.Jobs)
	require.True(t, ok)
	return j
}

// TestJobsDialogListsWorkspaceJobsNotTheLocalRegistry is the regression test
// for the client/server hole: the dialog called
// shell.GetBackgroundShellManager() from the TUI process, which under
// CRUSH_CLIENT_SERVER=1 is not the agent's process, so /jobs showed an empty
// list forever. The registry here holds a job the workspace does NOT report;
// if the dialog ever reads the registry again, that job shows up and this
// fails.
func TestJobsDialogListsWorkspaceJobsNotTheLocalRegistry(t *testing.T) {
	pinTTLs(t)

	manager := shell.GetBackgroundShellManager()
	local, err := manager.Start(context.Background(), t.TempDir(), nil, "local-registry-job", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = manager.Kill(local.ID)
		_ = manager.Remove(local.ID)
	})

	ws := &jobsWorkspace{countingWorkspace: countingWorkspace{ready: true}, jobs: stubJobs()}
	m := newBusyUI(&ws.countingWorkspace)
	m.com.Workspace = ws

	openJobs(t, m)

	require.Equal(t, 1, ws.listCalls, "the list must come from the workspace")
	j := openJobsDialogModel(t, m)
	require.Equal(t, ws.jobs, j.Jobs(),
		"the dialog must show exactly the workspace's jobs")
	for _, job := range j.Jobs() {
		require.NotEqual(t, local.ID, job.ID,
			"the TUI must not read the process-global shell registry; it is empty in client mode")
	}
}

// TestOpenJobsDialogDoesNoIOInUpdate pins that the fetch happens in the
// command, not on the Update goroutine. In client/server mode
// ListBackgroundJobs is an HTTP round-trip, and a synchronous one here
// freezes the render loop.
func TestOpenJobsDialogDoesNoIOInUpdate(t *testing.T) {
	pinTTLs(t)

	ws := &jobsWorkspace{countingWorkspace: countingWorkspace{ready: true}, jobs: stubJobs()}
	m := newBusyUI(&ws.countingWorkspace)
	m.com.Workspace = ws

	cmd := m.openJobsDialog()
	require.NotNil(t, cmd)
	require.Zero(t, ws.listCalls,
		"openJobsDialog must not fetch on the Update goroutine")
	require.Zero(t, ws.syncProbes(), "opening /jobs must make no synchronous workspace call")

	m.Update(cmd())
	require.Equal(t, 1, ws.listCalls, "the fetch belongs in the returned command")
}

// TestJobsDialogKillGoesThroughTheWorkspace pins the kill half. It used to
// call manager.Kill on the TUI's own registry, which in client mode holds
// nothing — the job was unkillable from the UI.
func TestJobsDialogKillGoesThroughTheWorkspace(t *testing.T) {
	pinTTLs(t)

	ws := &jobsWorkspace{countingWorkspace: countingWorkspace{ready: true}, jobs: stubJobs()}
	m := newBusyUI(&ws.countingWorkspace)
	m.com.Workspace = ws

	openJobs(t, m)
	require.NotNil(t, openJobsDialogModel(t, m))

	// Drive the real key path through the overlay: enter arms the
	// confirmation, "y" confirms. Which job the highlight tracks is pinned
	// by TestJobs_ConfirmKillTargetsHighlightedJob in the dialog package.
	require.Nil(t, m.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	cmd := m.handleDialogMsg(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	require.Empty(t, ws.killed, "the kill must not run on the Update goroutine")

	cmd()
	require.Equal(t, []string{"srv-a"}, ws.killed,
		"confirming must kill the highlighted job through the workspace")
	require.False(t, m.dialog.ContainsDialog(dialog.JobsID), "killing closes the dialog")
}

// TestOpenJobsDialogReportsFetchFailure pins that a client which cannot
// reach the server says so. Swallowing the error would render an empty job
// list, which is indistinguishable from "no jobs are running" — exactly the
// silent failure this change exists to remove.
func TestOpenJobsDialogReportsFetchFailure(t *testing.T) {
	pinTTLs(t)

	ws := &jobsWorkspace{
		countingWorkspace: countingWorkspace{ready: true},
		listErr:           context.DeadlineExceeded,
	}
	m := newBusyUI(&ws.countingWorkspace)
	m.com.Workspace = ws

	msg := m.openJobsDialog()()
	loaded, ok := msg.(jobsLoadedMsg)
	require.True(t, ok)
	require.Error(t, loaded.err)

	m.Update(loaded)
	require.False(t, m.dialog.ContainsDialog(dialog.JobsID),
		"a failed fetch must not open an empty jobs dialog that looks like 'no jobs'")
}
