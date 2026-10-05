package scheduler

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// dueClock returns a mutable clock for driving tasks due.
type dueClock struct{ t time.Time }

func (c *dueClock) now() time.Time { return c.t }

// servesOnly returns a serves predicate that only reports sessionID as
// served.
func servesOnly(sessionID string) func(string) bool {
	return func(s string) bool { return s == sessionID }
}

// TestDurableFireIsClaimedOnce is the core double-fire guard: with two
// stores on one file both serving the same session (two Crush windows
// showing one session), the on-disk claim lets exactly one of them fire
// a due durable task, and the loser sees it as not due.
func TestDurableFireIsClaimedOnce(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")
	clock := &dueClock{t: time.Date(2026, 10, 2, 10, 30, 0, 0, time.Local)}

	first := durableStore(t, path)
	first.now = clock.now
	require.NoError(t, first.Load())

	_, err := first.Create("s1", "* * * * *", "durable task", true, true)
	require.NoError(t, err)

	second := durableStore(t, path)
	second.now = clock.now
	require.NoError(t, second.Load())

	// Advance past the task's next run.
	clock.t = clock.t.Add(2 * time.Minute)

	firstDue := first.DueTasks(nil)
	require.Len(t, firstDue, 1, "the first store claims and fires the durable task")
	require.Equal(t, "durable task", firstDue[0].Prompt)

	// The claim advanced the task on disk, so the second store — even
	// though it also serves the session — must not get it back.
	secondDue := second.DueTasks(nil)
	require.Empty(t, secondDue, "the claim must keep the fire exactly-once across serving processes")

	// The claimed next run is claimWindow out, so the winner's own next
	// tick does not fire it again either.
	firstDueAgain := first.DueTasks(nil)
	require.Empty(t, firstDueAgain)
}

// TestUnservedSessionDefersDurableTask verifies a durable task only
// goes to a process serving its session: a window showing s2 must not
// fire s1's task, and the window showing s1 picks it up — the task is
// never lost and never runs where nobody is watching.
func TestUnservedSessionDefersDurableTask(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")
	clock := &dueClock{t: time.Date(2026, 10, 2, 10, 30, 0, 0, time.Local)}

	first := durableStore(t, path)
	first.now = clock.now
	require.NoError(t, first.Load())

	_, err := first.Create("s1", "* * * * *", "s1 durable task", true, true)
	require.NoError(t, err)

	second := durableStore(t, path)
	second.now = clock.now
	require.NoError(t, second.Load())
	_, err = second.Create("s2", "* * * * *", "s2 durable task", true, true)
	require.NoError(t, err)

	// Advance past both tasks' next run.
	clock.t = clock.t.Add(2 * time.Minute)

	// The window serving s2 gets only its own session's task; s1's task
	// stays due, unfired and unclaimed.
	secondDue := second.DueTasks(servesOnly("s2"))
	require.Len(t, secondDue, 1)
	require.Equal(t, "s2 durable task", secondDue[0].Prompt)

	durable, err := readDurableFile(path)
	require.NoError(t, err)
	s1Task, ok := lookupTask(durable, first.ListAll()[0].ID)
	require.True(t, ok)
	require.False(t, s1Task.NextRunAt.After(clock.t), "the unserved task must not be claimed or advanced")

	// The window serving s1 picks the deferred task up.
	firstDue := first.DueTasks(servesOnly("s1"))
	require.Len(t, firstDue, 1)
	require.Equal(t, "s1 durable task", firstDue[0].Prompt)
}

// TestConcurrentStoresMergeWrites verifies two processes sharing the
// persistence file do not clobber each other's durable tasks.
func TestConcurrentStoresMergeWrites(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")

	first := durableStore(t, path)
	require.NoError(t, first.Load())
	task1, err := first.Create("s1", "* * * * *", "first window task", true, true)
	require.NoError(t, err)

	second := durableStore(t, path)
	require.NoError(t, second.Load())
	_, err = second.Create("s2", "0 9 * * *", "second window task", true, true)
	require.NoError(t, err)

	// The second store's write must preserve the first store's task
	// instead of overwriting the file with only its own view.
	durable, err := readDurableFile(path)
	require.NoError(t, err)
	require.Len(t, durable, 2)

	// And the first store adopts the task the second window created, so
	// it still fires.
	_ = first.DueTasks(nil)
	found := false
	for _, task := range first.ListAll() {
		if task.ID != task1.ID && task.Prompt == "second window task" {
			found = true
		}
	}
	require.True(t, found, "a store must adopt tasks other processes created")
}

// TestDeletionPropagatesAcrossStores verifies a task deleted by a
// second window stops firing everywhere once the file changes.
func TestDeletionPropagatesAcrossStores(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")

	first := durableStore(t, path)
	require.NoError(t, first.Load())
	task, err := first.Create("s1", "* * * * *", "delete me", true, true)
	require.NoError(t, err)
	taskID := task.ID

	second := durableStore(t, path)
	require.NoError(t, second.Load())
	_, err = second.Delete("s1", taskID)
	require.NoError(t, err)

	_ = first.DueTasks(nil)
	require.Empty(t, first.ListAll(), "a task another process deleted must stop firing")
}
