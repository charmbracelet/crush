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

func TestSubscribeEventStreamParsesIDsAndResync(t *testing.T) {
	t.Parallel()

	payload := marshalSSEPayload(t)
	resync, err := json.Marshal(proto.Resync{Reason: proto.ResyncReasonEpochMismatch, LastEventID: "e-7"})
	require.NoError(t, err)
	resyncEnv, err := json.Marshal(pubsub.Payload{Type: pubsub.PayloadTypeResync, Payload: resync})
	require.NoError(t, err)

	gotHeader := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader <- r.Header.Get("Last-Event-ID")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "id: e-7\ndata: %s\n\n", resyncEnv)
		fmt.Fprintf(w, "data: %s\n\n", payload) // A snapshot with no id.
		fmt.Fprintf(w, "id: e-8\ndata: %s\n\n", payload)
		w.(http.Flusher).Flush()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events, err := captureClient(t, srv).SubscribeEventStream(ctx, "ws1", "old-3")
	require.NoError(t, err)
	require.Equal(t, "old-3", <-gotHeader)

	var got []StreamEvent
	for ev := range events {
		got = append(got, ev)
	}
	require.Len(t, got, 3)
	require.Equal(t, "e-7", got[0].ID)
	require.Equal(t, proto.Resync{Reason: proto.ResyncReasonEpochMismatch, LastEventID: "e-7"}, got[0].Payload)
	require.Empty(t, got[1].ID, "an id applies only to its own event")
	require.IsType(t, pubsub.Event[proto.AgentEvent]{}, got[1].Payload)
	require.Equal(t, "e-8", got[2].ID)
}

func TestSubscribeEventsIgnoresIDs(t *testing.T) {
	t.Parallel()

	payload := marshalSSEPayload(t)
	gotHeader := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader <- r.Header.Get("Last-Event-ID")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "id: e-1\ndata: %s\n\n", payload)
		w.(http.Flusher).Flush()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events, err := captureClient(t, srv).SubscribeEvents(ctx, "ws1")
	require.NoError(t, err)
	require.Empty(t, <-gotHeader)
	ev, ok := <-events
	require.True(t, ok)
	require.IsType(t, pubsub.Event[proto.AgentEvent]{}, ev)
	_, ok = <-events
	require.False(t, ok)
}
