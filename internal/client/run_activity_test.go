package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestSubscribeEventsDecodesRunActivity(t *testing.T) {
	t.Parallel()

	activity := proto.RunActivity{
		SessionID: "session",
		RunID:     "run",
		MessageID: "assistant",
		Type:      proto.RunActivityToolStarted,
		ToolCall: &proto.ToolCall{
			ID:       "call",
			Name:     "view",
			Input:    `{"file_path":"main.go"}`,
			Finished: true,
		},
	}
	eventPayload, err := json.Marshal(pubsub.Event[proto.RunActivity]{
		Type:    pubsub.UpdatedEvent,
		Payload: activity,
	})
	require.NoError(t, err)
	payload, err := json.Marshal(pubsub.Payload{
		Type:    pubsub.PayloadTypeRunActivity,
		Payload: eventPayload,
	})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: " + string(payload) + "\n\n"))
	}))
	t.Cleanup(server.Close)
	events, err := captureClient(t, server).SubscribeEvents(t.Context(), "workspace")
	require.NoError(t, err)

	select {
	case raw := <-events:
		event, ok := raw.(pubsub.Event[proto.RunActivity])
		require.True(t, ok)
		require.Equal(t, activity, event.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for run activity")
	}
}
