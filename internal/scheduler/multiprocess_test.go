package scheduler

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNonOwnerMergeDoesNotResurrectStaleTasks guards the merge's dirty
// set: a non-owner process holds in-memory copies of durable tasks as of
// its own last sync, and its merge writes must not let those stale
// copies win over the owner's newer NextRunAt / RunCount. Here the
// non-owner deletes an unrelated durable task, which forces a merge
// write while it still holds a pre-fire copy of the fired task.
func TestNonOwnerMergeDoesNotResurrectStaleTasks(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")
	clock := &dueClock{t: time.Date(2026, 10, 2, 10, 30, 0, 0, time.Local)}

	owner := durableStore(t, path)
	owner.now = clock.now
	require.NoError(t, owner.Load())

	fired, err := owner.Create("s1", "* * * * *", "fired task", true, true)
	require.NoError(t, err)
	victim, err := owner.Create("s1", "* * * * *", "deleted task", true, true)
	require.NoError(t, err)

	second := durableStore(t, path)
	second.now = clock.now
	require.NoError(t, second.Load())

	// The owner fires one task: its disk copy advances.
	owner.MarkFired(fired.ID)
	firedAfter, ok := lookupTask(owner.List("s1"), fired.ID)
	require.True(t, ok)
	require.Equal(t, 1, firedAfter.RunCount, "the fired task must show its run")

	// The non-owner deletes the other task, forcing a merge write while
	// its copy of the fired task is stale (RunCount 0, past NextRunAt).
	_, err = second.Delete("s1", victim.ID)
	require.NoError(t, err)

	reloaded := durableStore(t, path)
	reloaded.now = clock.now
	require.NoError(t, reloaded.Load())
	tasks := reloaded.ListAll()
	require.Len(t, tasks, 1)
	require.Equal(t, fired.ID, tasks[0].ID)
	require.Equal(t, firedAfter.RunCount, tasks[0].RunCount, "the owner's fired copy must win over the non-owner's stale one")
	require.True(t, firedAfter.NextRunAt.Equal(tasks[0].NextRunAt),
		"the owner's fired NextRunAt must win over the non-owner's stale one: want %s, got %s",
		firedAfter.NextRunAt, tasks[0].NextRunAt)
}

// TestNonOwnerListRefreshesFromDisk verifies a non-owner's CronList
// reflects what actually happened elsewhere: after the owner fires a
// one-shot (deleting it) and advances a recurring task, the non-owner
// must not keep listing the pre-fire copies.
func TestNonOwnerListRefreshesFromDisk(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")
	clock := &dueClock{t: time.Date(2026, 10, 2, 10, 30, 0, 0, time.Local)}

	owner := durableStore(t, path)
	owner.now = clock.now
	require.NoError(t, owner.Load())

	oneShot, err := owner.Create("s1", "45 10 2 10 *", "once", false, true)
	require.NoError(t, err)
	recurring, err := owner.Create("s1", "* * * * *", "again", true, true)
	require.NoError(t, err)

	second := durableStore(t, path)
	second.now = clock.now
	require.NoError(t, second.Load())
	require.Len(t, second.List("s1"), 2)

	// The owner fires both: the one-shot is deleted, the recurring task
	// is rescheduled.
	owner.MarkFired(oneShot.ID)
	owner.MarkFired(recurring.ID)

	tasks := second.List("s1")
	require.Len(t, tasks, 1, "the fired one-shot must not still be listed")
	require.Equal(t, recurring.ID, tasks[0].ID)
	require.Equal(t, 1, tasks[0].RunCount, "the listed recurring task must be the owner's post-fire copy")
}

// TestSetLastErrorKeepsSchedule verifies SetLastError, used to record a
// failure of the run a fire already dispatched: unlike MarkError it must
// not reschedule (MarkFired already advanced the schedule) or delete
// anything — only stamp LastError so CronList shows it.
func TestSetLastErrorKeepsSchedule(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")
	clock := &dueClock{t: time.Date(2026, 10, 2, 10, 30, 0, 0, time.Local)}

	store := durableStore(t, path)
	store.now = clock.now
	require.NoError(t, store.Load())

	task, err := store.Create("s1", "* * * * *", "ping", true, true)
	require.NoError(t, err)

	store.MarkFired(task.ID)
	afterFire := store.List("s1")[0]
	require.Empty(t, afterFire.LastError)

	store.SetLastError(task.ID, errRunFailed)

	tasks := store.List("s1")
	require.Len(t, tasks, 1)
	require.Equal(t, errRunFailed.Error(), tasks[0].LastError)
	require.True(t, afterFire.NextRunAt.Equal(tasks[0].NextRunAt),
		"SetLastError must not reschedule: want %s, got %s",
		afterFire.NextRunAt, tasks[0].NextRunAt)
	require.Equal(t, afterFire.RunCount, tasks[0].RunCount)

	// The error must reach disk too, so another window's CronList sees it.
	reloaded := durableStore(t, path)
	require.NoError(t, reloaded.Load())
	reloadedTasks := reloaded.List("s1")
	require.Len(t, reloadedTasks, 1)
	require.Equal(t, errRunFailed.Error(), reloadedTasks[0].LastError)
}

var errRunFailed = &staticError{"the model call failed"}

type staticError struct{ msg string }

func (e *staticError) Error() string { return e.msg }

// TestSessionOnlyTasksNeverTouchDisk verifies the persist gate: a task
// that never needed persistence must never trigger a file
// read-modify-write, so a disk or lock problem cannot block creating,
// firing, or deleting it.
func TestSessionOnlyTasksNeverTouchDisk(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")
	store := NewStore(path)

	task, err := store.Create("s1", "* * * * *", "session-only", true, false)
	require.NoError(t, err)
	require.NoFileExists(t, path)

	store.MarkFired(task.ID)
	require.NoFileExists(t, path)

	taskAfterFire := store.List("s1")[0]
	_, err = store.Delete("s1", taskAfterFire.ID)
	require.NoError(t, err)
	require.NoFileExists(t, path)
}
