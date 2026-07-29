package dialog

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// testJobs builds n job DTOs, oldest first, the shape the workspace
// returns. The dialog is now fed its list instead of reading the
// process-global background shell registry, so these tests no longer start
// real shells: under CRUSH_CLIENT_SERVER=1 that registry lives in the agent
// process and the dialog can never see it.
func testJobs(n int) []proto.BackgroundJob {
	base := time.Now().Add(-time.Duration(n) * time.Minute)
	jobs := make([]proto.BackgroundJob, 0, n)
	for i := range n {
		jobs = append(jobs, proto.BackgroundJob{
			ID:        fmt.Sprintf("%03X", i+1),
			Command:   fmt.Sprintf("sleep %d", i),
			StartedAt: base.Add(time.Duration(i) * time.Second),
		})
	}
	return jobs
}

func newTestJobs(t *testing.T, jobs []proto.BackgroundJob) *Jobs {
	t.Helper()

	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	j, err := NewJobs(com, jobs)
	require.NoError(t, err)
	return j
}

// TestJobs_ListsWorkspaceJobs pins that the dialog renders exactly the jobs
// it was handed. It is the local half of the client/server wiring: the
// dialog must be a pure view over the workspace's list.
func TestJobs_ListsWorkspaceJobs(t *testing.T) {
	jobs := testJobs(3)
	j := newTestJobs(t, jobs)

	require.Equal(t, len(jobs), j.list.Len())
	for i, want := range jobs {
		j.list.SetSelected(i)
		got, ok := j.selectedJob()
		require.True(t, ok)
		require.Equal(t, want.ID, got.ID)
		require.Equal(t, want.Command, got.Command)
	}
}

// TestJobs_EnterAsksBeforeKilling pins that enter does not terminate a job
// outright. Killing a background job is destructive and irreversible — the
// dialog's own ctrl+x path asks "Kill this job?" first — yet enter used to
// return ActionKillJob straight from the key handler while the help text
// still advertised it as "choose". A user arrowing through the list and
// pressing enter to look at a job would kill it instead.
func TestJobs_EnterAsksBeforeKilling(t *testing.T) {
	jobs := testJobs(2)

	j := newTestJobs(t, jobs)
	require.Equal(t, len(jobs), j.list.Len())
	j.list.SetSelected(1)

	action := j.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.NotEqual(t, ActionKillJob{ShellID: jobs[1].ID}, action,
		"enter must not kill the highlighted job without confirmation")
	require.Nil(t, action, "enter must only arm the confirmation, not act")
	require.Equal(t, jobsModeKilling, j.mode, "enter must arm the kill confirmation")
}

// TestJobs_ConfirmKillTargetsHighlightedJob covers the other direction: the
// confirmation must still kill, and it must kill the job the user
// highlighted. Entering kill mode calls refresh(), which rebuilds the list
// while list.SetItems keeps the selected *index* — so this also pins that a
// rebuild does not reshuffle underneath the cursor.
func TestJobs_ConfirmKillTargetsHighlightedJob(t *testing.T) {
	jobs := testJobs(6)

	j := newTestJobs(t, jobs)
	require.Equal(t, len(jobs), j.list.Len())
	j.list.SetSelected(4)

	require.Nil(t, j.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	action := j.HandleMsg(tea.KeyPressMsg{Code: 'y', Text: "y"})

	require.Equal(t, ActionKillJob{ShellID: jobs[4].ID}, action,
		"confirming must kill the highlighted job, not another one")
	require.Equal(t, jobsModeNormal, j.mode)
}

// TestJobs_KillRespectsTheActiveFilter pins that filtering then killing
// terminates the job the user can actually see.
//
// FilterableList.SetItems installs the *unfiltered* set, so a refresh that
// did not re-apply the query widened the list back to every job while
// list.SetSelected kept the same index — arming a kill on a filtered list
// then confirming it killed whichever job happened to sit at that index in
// the full list. Selection is restored by job ID for the same reason.
func TestJobs_KillRespectsTheActiveFilter(t *testing.T) {
	jobs := testJobs(6)
	j := newTestJobs(t, jobs)

	// Filter down to exactly the last job; it is at index 5 unfiltered and
	// index 0 filtered, so an index-preserving rebuild lands on jobs[0].
	for _, r := range jobs[5].Command {
		j.HandleMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	require.Equal(t, 1, len(j.list.FilteredItems()), "the filter must isolate one job")

	require.Nil(t, j.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.Equal(t, 1, len(j.list.FilteredItems()),
		"arming a kill must not widen the filtered list back to every job")

	action := j.HandleMsg(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.Equal(t, ActionKillJob{ShellID: jobs[5].ID}, action,
		"the kill must target the filtered job the user was looking at")
}

// TestJobs_FirstRowIsSelectedOnOpen pins that /jobs is usable immediately.
// list.SetSelected(0) runs in the constructor before any item exists and so
// leaves the selection at -1; without restoring it after the items are
// built, the first ctrl+x answers "No job selected" with jobs on screen.
func TestJobs_FirstRowIsSelectedOnOpen(t *testing.T) {
	jobs := testJobs(3)
	j := newTestJobs(t, jobs)

	got, ok := j.selectedJob()
	require.True(t, ok, "opening /jobs with jobs listed must highlight a row")
	require.Equal(t, jobs[0].ID, got.ID)

	action := j.HandleMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	require.Nil(t, action, "ctrl+x must arm the confirmation, not report 'No job selected'")
	require.Equal(t, jobsModeKilling, j.mode)
}

// TestJobs_CancelKillDoesNotKill pins that answering "n" leaves every job
// alone.
func TestJobs_CancelKillDoesNotKill(t *testing.T) {
	j := newTestJobs(t, testJobs(2))
	j.list.SetSelected(0)

	require.Nil(t, j.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.Equal(t, jobsModeKilling, j.mode)
	action := j.HandleMsg(tea.KeyPressMsg{Code: 'n', Text: "n"})

	require.Nil(t, action, "cancelling must not produce a kill action")
	require.Equal(t, jobsModeNormal, j.mode)
}

// TestJobs_KillKeysNeedASelection pins that neither kill key arms the
// "Kill this job?" confirmation when there is nothing to kill. An empty job
// list used to answer ctrl+x with a confirmation prompt for no job at all.
func TestJobs_KillKeysNeedASelection(t *testing.T) {
	for name, msg := range map[string]tea.KeyPressMsg{
		"ctrl+x": {Code: 'x', Mod: tea.ModCtrl},
		"enter":  {Code: tea.KeyEnter},
	} {
		t.Run(name, func(t *testing.T) {
			// A fresh dialog per case: sharing one would let the first
			// subtest's mode leak into the second and fail it for the
			// wrong reason.
			j := newTestJobs(t, nil)
			require.Zero(t, j.list.Len(), "no jobs must be registered for this case")

			action := j.HandleMsg(msg)
			require.IsType(t, ActionCmd{}, action, "%s must report instead of arming a kill", name)
			require.Equal(t, jobsModeNormal, j.mode,
				"%s must not show a kill confirmation with nothing selected", name)
		})
	}
}
