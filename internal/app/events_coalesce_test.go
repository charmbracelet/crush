package app

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// TestEventsCoalesceMessageUpdates checks that the fan-in into app.events
// bounds pending message updates for a stalled subscriber while keeping
// must-deliver updates and per-message ordering.
func TestEventsCoalesceMessageUpdates(t *testing.T) {
	t.Parallel()

	app := NewForTest(t.Context())
	t.Cleanup(app.ShutdownForTest)
	msgs := pubsub.NewCoalescingBroker(message.CoalesceKey)
	t.Cleanup(msgs.Shutdown)
	app.subscribe(app.eventsCtx, "messages", msgs.Subscribe)
	events := app.Events(t.Context())
	require.Eventually(t, func() bool { return msgs.GetSubscriberCount() == 1 }, time.Second, time.Millisecond)

	const n = 1000
	msgs.Publish(pubsub.CreatedEvent, message.Message{ID: "a"})
	msgs.Publish(pubsub.CreatedEvent, message.Message{ID: "b"})
	for v := int64(1); v <= n; v++ {
		if v == n/2 {
			msgs.PublishMustDeliver(t.Context(), pubsub.UpdatedEvent, message.Message{ID: "a", UpdatedAt: v})
			continue
		}
		msgs.Publish(pubsub.UpdatedEvent, message.Message{ID: "a", UpdatedAt: v})
		msgs.Publish(pubsub.UpdatedEvent, message.Message{ID: "b", UpdatedAt: v})
	}
	msgs.Publish(pubsub.DeletedEvent, message.Message{ID: "b"})

	var got []pubsub.Event[message.Message]
	for {
		select {
		case ev := <-events:
			inner, ok := ev.Payload.(pubsub.Event[message.Message])
			require.True(t, ok)
			got = append(got, inner)
			continue
		case <-time.After(200 * time.Millisecond):
		}
		break
	}

	last := map[string]int64{}
	created := map[string]bool{}
	deleted := map[string]bool{}
	var sawMustDeliver bool
	for _, ev := range got {
		id := ev.Payload.ID
		switch ev.Type {
		case pubsub.CreatedEvent:
			created[id] = true
		case pubsub.UpdatedEvent:
			require.True(t, created[id], "update before create for %s", id)
			require.False(t, deleted[id], "update after delete for %s", id)
			require.Greater(t, ev.Payload.UpdatedAt, last[id], "older snapshot after newer for %s", id)
			last[id] = ev.Payload.UpdatedAt
			if ev.Payload.UpdatedAt == n/2 {
				require.True(t, ev.MustDeliver)
				sawMustDeliver = true
			}
		case pubsub.DeletedEvent:
			deleted[id] = true
		}
	}
	require.True(t, sawMustDeliver, "must-deliver update was coalesced away")
	require.Equal(t, int64(n), last["a"])
	require.Equal(t, int64(n), last["b"])
	require.True(t, deleted["b"])
	require.Less(t, len(got), 20, "stalled subscriber should hold a handful of events, not %d", 2*n)
}
