package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestSendEventAfterContextCancelIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events := make(chan any, 1)
	require.False(t, sendEvent(ctx, events, "one"))
	require.False(t, sendEvent(ctx, events, "two"))

	select {
	case ev := <-events:
		require.Failf(t, "unexpected event", "event: %v", ev)
	default:
	}
}

func TestSubscribeEventsContextCancelClosesEvents(t *testing.T) {
	t.Parallel()

	payload := marshalSSEPayload(t)
	firstEventSent := make(chan struct{})
	writeSecondEvent := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		_, err := fmt.Fprintf(w, "data: %s\n\n", payload)
		require.NoError(t, err)
		flusher.Flush()
		close(firstEventSent)

		select {
		case <-writeSecondEvent:
		case <-time.After(5 * time.Second):
			return
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := captureClient(t, srv)
	events, err := c.SubscribeEvents(ctx, "ws1")
	require.NoError(t, err)

	select {
	case <-firstEventSent:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for server event")
	}

	select {
	case <-events:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for first event")
	}

	cancel()
	close(writeSecondEvent)

	select {
	case _, ok := <-events:
		require.False(t, ok)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for event channel close")
	}
}

func marshalSSEPayload(t *testing.T) []byte {
	t.Helper()

	eventPayload, err := json.Marshal(pubsub.Event[proto.AgentEvent]{
		Type: pubsub.CreatedEvent,
		Payload: proto.AgentEvent{
			Type: proto.AgentEventTypeResponse,
		},
	})
	require.NoError(t, err)

	payload, err := json.Marshal(pubsub.Payload{
		Type:    pubsub.PayloadTypeAgentEvent,
		Payload: eventPayload,
	})
	require.NoError(t, err)
	return payload
}
