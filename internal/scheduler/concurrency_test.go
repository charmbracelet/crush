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

// TestStoreOwnershipIsExclusive verifies exactly one process owns the
// scheduler: the first store to load takes the ownership lock, a second
// store on the same file does not, and the lock transfers once the
// owner releases it.
func TestStoreOwnershipIsExclusive(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")

	owner := NewStore(path)
	require.NoError(t, owner.Load())
	require.True(t, owner.Owner(), "the first store must own firing")

	second := NewStore(path)
	require.NoError(t, second.Load())
	require.False(t, second.Owner(), "a second store must not own firing")

	owner.Close()
	require.False(t, owner.Owner())

	third := NewStore(path)
	require.NoError(t, third.Load())
	require.True(t, third.Owner(), "ownership must transfer after release")
}

// TestNonOwnerSkipsDurableTasks is the core double-fire guard: with two
// stores on one file (two Crush windows on one project), only the owner
// sees durable tasks as due, while each process still fires its own
// session-only tasks.
func TestNonOwnerSkipsDurableTasks(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")
	clock := &dueClock{t: time.Date(2026, 10, 2, 10, 30, 0, 0, time.Local)}

	owner := NewStore(path)
	owner.now = clock.now
	require.NoError(t, owner.Load())

	_, err := owner.Create("s1", "* * * * *", "durable task", true, true)
	require.NoError(t, err)

	second := NewStore(path)
	second.now = clock.now
	require.NoError(t, second.Load())
	require.False(t, second.Owner())

	_, err = second.Create("s2", "* * * * *", "in-memory task", true, false)
	require.NoError(t, err)

	// Advance past both tasks' next run.
	clock.t = clock.t.Add(2 * time.Minute)

	ownerDue := owner.DueTasks()
	require.Len(t, ownerDue, 1, "the owner fires the durable task")
	require.Equal(t, "durable task", ownerDue[0].Prompt)

	secondDue := second.DueTasks()
	require.Len(t, secondDue, 1, "the non-owner fires only its in-memory task")
	require.Equal(t, "in-memory task", secondDue[0].Prompt)

	// Ownership is retried, so a non-owner takes over once the owner is
	// gone rather than waiting for a restart.
	owner.Close()
	second.lastOwnerAttempt = time.Now().Add(-ownershipRetryInterval)
	secondDue = second.DueTasks()
	require.True(t, second.Owner(), "the surviving store must take over firing")
	require.Len(t, secondDue, 2, "the new owner fires the durable task and its own in-memory task")
}

// TestConcurrentStoresMergeWrites verifies two processes sharing the
// persistence file do not clobber each other's durable tasks.
func TestConcurrentStoresMergeWrites(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")

	owner := NewStore(path)
	require.NoError(t, owner.Load())
	task1, err := owner.Create("s1", "* * * * *", "owner task", true, true)
	require.NoError(t, err)

	second := NewStore(path)
	require.NoError(t, second.Load())
	_, err = second.Create("s2", "0 9 * * *", "second window task", true, true)
	require.NoError(t, err)

	// The non-owner's write must preserve the owner's task instead of
	// overwriting the file with only its own view.
	durable, err := readDurableFile(path)
	require.NoError(t, err)
	require.Len(t, durable, 2)

	// And the owner adopts the task the second window created, so it
	// still fires.
	_ = owner.DueTasks()
	found := false
	for _, task := range owner.ListAll() {
		if task.ID != task1.ID && task.Prompt == "second window task" {
			found = true
		}
	}
	require.True(t, found, "the owner must adopt tasks other processes created")
}

// TestDeletionPropagatesToOwner verifies a task deleted by a second
// window stops firing for the owner once the file changes.
func TestDeletionPropagatesToOwner(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")

	owner := NewStore(path)
	require.NoError(t, owner.Load())
	task, err := owner.Create("s1", "* * * * *", "delete me", true, true)
	require.NoError(t, err)
	taskID := task.ID

	second := NewStore(path)
	require.NoError(t, second.Load())
	_, err = second.Delete("s1", taskID)
	require.NoError(t, err)

	_ = owner.DueTasks()
	require.Empty(t, owner.ListAll(), "the owner must stop firing a task another process deleted")
}
