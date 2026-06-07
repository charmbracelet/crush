package subagents

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestRuntime_Register(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	t.Cleanup(rt.Shutdown)

	entry := rt.Register("parent-1", "child-1", "my-agent", "blue", "")

	require.Equal(t, "parent-1", entry.ParentSessionID)
	require.Equal(t, "child-1", entry.ChildSessionID)
	require.Equal(t, "my-agent", entry.Name)
	require.Equal(t, "blue", entry.Color)
	require.Equal(t, StatusRunning, entry.Status)
	require.False(t, entry.StartedAt.IsZero(), "StartedAt must be set")

	entries := rt.List("parent-1")
	require.Len(t, entries, 1)
	require.Equal(t, entry, entries[0])
}

func TestRuntime_SetStatus(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	t.Cleanup(rt.Shutdown)

	rt.Register("parent-1", "child-1", "my-agent", "green", "")
	rt.SetStatus("child-1", "queued")

	entries := rt.List("parent-1")
	require.Len(t, entries, 1)
	require.Equal(t, "queued", entries[0].Status)
}

func TestRuntime_List_IsolatedByParent(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	t.Cleanup(rt.Shutdown)

	rt.Register("parent-A", "child-A", "agent-a", "cyan", "")
	rt.Register("parent-B", "child-B", "agent-b", "magenta", "")

	entriesA := rt.List("parent-A")
	require.Len(t, entriesA, 1)
	require.Equal(t, "child-A", entriesA[0].ChildSessionID)

	entriesB := rt.List("parent-B")
	require.Len(t, entriesB, 1)
	require.Equal(t, "child-B", entriesB[0].ChildSessionID)
}

func TestRuntime_List_ReturnsCopy(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	t.Cleanup(rt.Shutdown)

	rt.Register("parent-1", "child-1", "my-agent", "yellow", "")

	first := rt.List("parent-1")
	require.Len(t, first, 1)

	// Mutate the returned slice.
	first[0] = RunningEntry{ChildSessionID: "mutated"}
	first = append(first, RunningEntry{ChildSessionID: "extra"})

	// Internal state must be unaffected.
	second := rt.List("parent-1")
	require.Len(t, second, 1)
	require.Equal(t, "child-1", second[0].ChildSessionID)
}

func TestRuntime_Subscribe_ReceivesRegisterEvent(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	t.Cleanup(rt.Shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ch := rt.Subscribe(ctx)

	rt.Register("parent-1", "child-1", "my-agent", "blue", "")

	select {
	case ev := <-ch:
		require.Equal(t, "parent-1", ev.Payload.ParentSessionID)
		require.Len(t, ev.Payload.Entries, 1)
		require.Equal(t, "child-1", ev.Payload.Entries[0].ChildSessionID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for register event")
	}
}

func TestRuntime_Finish_RemovesEntry(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	t.Cleanup(rt.Shutdown)

	rt.Register("parent-1", "child-1", "my-agent", "blue", "")
	rt.Finish("child-1", StatusCompleted)

	require.Empty(t, rt.List("parent-1"))
}

func TestRuntime_Finish_PublishesFinishedEvent(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	t.Cleanup(rt.Shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	rt.Register("parent-1", "child-1", "my-agent", "blue", "claude")

	ch := rt.Subscribe(ctx)

	rt.Finish("child-1", StatusCompleted)

	select {
	case ev := <-ch:
		require.NotNil(t, ev.Payload.Finished, "Finished must be set when sub-agent finishes")
		require.Equal(t, "child-1", ev.Payload.Finished.ChildSessionID)
		require.Equal(t, StatusCompleted, ev.Payload.Finished.Status)
		require.Empty(t, ev.Payload.Entries)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for finish event")
	}
}

func TestRuntime_Finish_StatusFlowsThrough(t *testing.T) {
	t.Parallel()

	cases := []string{StatusCompleted, StatusCancelled, StatusFailed}
	for _, status := range cases {
		t.Run(status, func(t *testing.T) {
			t.Parallel()

			rt := NewRuntime()
			t.Cleanup(rt.Shutdown)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			rt.Register("parent-1", "child-1", "agent", "red", "")
			ch := rt.Subscribe(ctx)

			rt.Finish("child-1", status)

			select {
			case ev := <-ch:
				require.NotNil(t, ev.Payload.Finished)
				require.Equal(t, status, ev.Payload.Finished.Status)
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for finish event with status %q", status)
			}
		})
	}
}

func TestRuntime_Finish_UnknownChildIsNoOp(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	t.Cleanup(rt.Shutdown)

	require.NotPanics(t, func() {
		rt.Finish("missing", StatusCompleted)
	})
}

func TestRuntime_NilSafe(t *testing.T) {
	t.Parallel()

	var rt *Runtime

	require.NotPanics(t, func() {
		rt.Register("parent-1", "child-1", "agent", "red", "")
	})
	require.NotPanics(t, func() {
		rt.Finish("child-1", StatusCompleted)
	})
	require.NotPanics(t, func() {
		rt.SetStatus("child-1", "queued")
	})
	require.NotPanics(t, func() {
		entries := rt.List("parent-1")
		require.Nil(t, entries)
	})
	require.NotPanics(t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := rt.Subscribe(ctx)
		// Channel must be closed (nil Runtime acts like a shut-down broker).
		select {
		case _, ok := <-ch:
			require.False(t, ok, "Subscribe on nil Runtime must return a closed channel")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Subscribe on nil Runtime did not return a closed channel")
		}
	})
	require.NotPanics(t, func() {
		rt.Shutdown()
	})
}

func TestRuntime_Shutdown(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ch := rt.Subscribe(ctx)
	rt.Shutdown()

	select {
	case _, ok := <-ch:
		require.False(t, ok, "channel must be closed after Shutdown")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel to close after Shutdown")
	}
}

func TestRuntime_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	rt := NewRuntime()
	t.Cleanup(rt.Shutdown)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			childID := "child-" + string(rune('A'+i))
			rt.Register("parent-shared", childID, "agent", "white", "")
			rt.List("parent-shared")
			rt.SetStatus(childID, "queued")
			rt.List("parent-shared")
			rt.Finish(childID, StatusCompleted)
		}(i)
	}

	wg.Wait()
}

// Compile-time assertion: Subscribe must return the correct channel type.
var _ <-chan pubsub.Event[RuntimeEvent] = (*Runtime)(nil).Subscribe(context.Background())
