package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// TestCancelOwnQuestion_CancelsOnlyItsSession covers `crush run` sharing a
// workspace agent that offers the question tool: a question raised in the
// run's own session is cancelled, since nobody can answer it, while
// questions from other clients' sessions and other events are left alone.
func TestCancelOwnQuestion_CancelsOnlyItsSession(t *testing.T) {
	t.Parallel()

	var cancels atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws/questions/cancel" {
			http.NotFound(w, r)
			return
		}
		cancels.Add(1)
		_ = json.NewEncoder(w).Encode(proto.QuestionAnswerResponse{Resolved: true})
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	question := func(sessionID string) pubsub.Event[proto.QuestionRequest] {
		return pubsub.Event[proto.QuestionRequest]{
			Type:    pubsub.CreatedEvent,
			Payload: proto.QuestionRequest{ID: "batch", SessionID: sessionID},
		}
	}

	cancelOwnQuestion(t.Context(), c, "ws", "run", question("other"))
	cancelOwnQuestion(t.Context(), c, "ws", "run", pubsub.Event[proto.Message]{Payload: proto.Message{SessionID: "run"}})
	require.Zero(t, cancels.Load(), "only the run's own questions may be cancelled")

	cancelOwnQuestion(t.Context(), c, "ws", "run", question("run"))
	require.EqualValues(t, 1, cancels.Load(), "a question in the run's session must be cancelled")
}
