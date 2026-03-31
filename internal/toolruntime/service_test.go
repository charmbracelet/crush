package toolruntime

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestDeleteSessionRemovesAllStatesAndPublishesDeletedEvents(t *testing.T) {
	t.Parallel()

	svc := NewService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := svc.Subscribe(ctx)
	first := State{SessionID: "session-1", ToolCallID: "call-1", ToolName: "bash", Status: StatusRunning}
	second := State{SessionID: "session-1", ToolCallID: "call-2", ToolName: "write", Status: StatusCompleted}
	other := State{SessionID: "session-2", ToolCallID: "call-9", ToolName: "read", Status: StatusPending}

	svc.Publish(first)
	svc.Publish(second)
	svc.Publish(other)

	drainUpdatedEvents(t, events, 3)

	svc.DeleteSession("session-1")

	deleted := collectDeletedEvents(t, events, 2)
	require.ElementsMatch(t, []string{"call-1", "call-2"}, []string{deleted[0].ToolCallID, deleted[1].ToolCallID})

	_, ok := svc.Get("session-1", "call-1")
	require.False(t, ok)
	_, ok = svc.Get("session-1", "call-2")
	require.False(t, ok)

	remaining, ok := svc.Get("session-2", "call-9")
	require.True(t, ok)
	require.Equal(t, "call-9", remaining.ToolCallID)
}

func TestDeleteSessionNoopWhenSessionMissing(t *testing.T) {
	t.Parallel()

	svc := NewService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := svc.Subscribe(ctx)

	svc.DeleteSession("missing")

	select {
	case evt := <-events:
		t.Fatalf("unexpected event: %#v", evt)
	default:
	}
}

func drainUpdatedEvents(t *testing.T, events <-chan pubsub.Event[State], count int) {
	t.Helper()
	for range count {
		evt := <-events
		require.Equal(t, pubsub.UpdatedEvent, evt.Type)
	}
}

func collectDeletedEvents(t *testing.T, events <-chan pubsub.Event[State], count int) []State {
	t.Helper()
	deleted := make([]State, 0, count)
	for len(deleted) < count {
		evt := <-events
		require.Equal(t, pubsub.DeletedEvent, evt.Type)
		deleted = append(deleted, evt.Payload)
	}
	return deleted
}
