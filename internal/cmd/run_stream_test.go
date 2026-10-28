package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// newRunServer serves just enough of the server API for runNonInteractive
// to start a run in workspace "w". Once the prompt is accepted, the event
// stream is handed to stream, which decides how the run ends as far as the
// client can tell.
func newStreamRunServer(t *testing.T, stream func(w http.ResponseWriter, r *http.Request, runID string)) *client.Client {
	t.Helper()

	accepted := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/workspaces/w/agent":
			_ = json.NewEncoder(w).Encode(proto.AgentInfo{IsReady: true})
		case "POST /v1/workspaces/w/agent/update":
			w.WriteHeader(http.StatusOK)
		case "POST /v1/workspaces/w/sessions":
			_ = json.NewEncoder(w).Encode(proto.Session{ID: "s"})
		case "POST /v1/workspaces/w/agent":
			var msg proto.AgentMessage
			if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			accepted <- msg.RunID
			w.WriteHeader(http.StatusAccepted)
		case "GET /v1/workspaces/w/events":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_ = http.NewResponseController(w).Flush()
			select {
			case runID := <-accepted:
				stream(w, r, runID)
			case <-r.Context().Done():
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	return c
}

// writeRunComplete sends rc on the event stream the way the server does.
func writeRunComplete(t *testing.T, w http.ResponseWriter, rc proto.RunComplete) {
	inner, err := json.Marshal(pubsub.Event[proto.RunComplete]{Type: pubsub.UpdatedEvent, Payload: rc})
	if err != nil {
		t.Error(err)
		return
	}
	data, err := json.Marshal(pubsub.Payload{Type: pubsub.PayloadTypeRunComplete, Payload: inner})
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	_ = http.NewResponseController(w).Flush()
}

// runAgainst runs `crush run hi` in workspace "w" on the server c talks to.
func runAgainst(t *testing.T, c *client.Client) error {
	t.Helper()
	ws := &proto.Workspace{ID: "w", Config: &config.Config{Options: &config.Options{}}}
	return runNonInteractive(t.Context(), c, ws, "hi", "", "", "", true, "", false)
}

// TestRunNonInteractive_LostStreamIsAnError is the regression test for
// `crush run` exiting 0 with nothing on stdout when the server died or the
// event stream ended mid-run. The run never completed, so a script chaining
// on the exit status must see a failure.
func TestRunNonInteractive_LostStreamIsAnError(t *testing.T) {
	t.Parallel()

	for name, end := range map[string]func(t *testing.T, w http.ResponseWriter){
		// The server stops streaming, e.g. because the workspace was torn
		// down under the run.
		"stream ends": func(*testing.T, http.ResponseWriter) {},
		// The server process is killed: the connection drops without the
		// response being terminated.
		"connection drops": func(t *testing.T, w http.ResponseWriter) {
			conn, _, err := http.NewResponseController(w).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := newStreamRunServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
				end(t, w)
			})
			require.ErrorContains(t, runAgainst(t, c), "lost connection",
				"a run whose event stream was lost must not succeed")
		})
	}
}

// TestRunNonInteractive_ServerCancelledRunIsAnError covers the graceful
// variant: a server that shuts down cancels the run (as does a workspace
// teardown or another client) and still sends its RunComplete. Its text is
// whatever the agent had written so far, so exiting 0 would pass truncated
// output off as the answer.
func TestRunNonInteractive_ServerCancelledRunIsAnError(t *testing.T) {
	t.Parallel()

	c := newStreamRunServer(t, func(w http.ResponseWriter, r *http.Request, runID string) {
		writeRunComplete(t, w, proto.RunComplete{
			SessionID: "s",
			RunID:     runID,
			MessageID: "m",
			Text:      "partial answer",
			Error:     "context canceled",
			Cancelled: true,
		})
		<-r.Context().Done()
	})
	require.ErrorContains(t, runAgainst(t, c), "cancelled",
		"a run cancelled by someone else must not succeed")
}

// TestRunNonInteractive_CompletedRunSucceeds is the control: a run that
// completes still exits cleanly.
func TestRunNonInteractive_CompletedRunSucceeds(t *testing.T) {
	t.Parallel()

	c := newStreamRunServer(t, func(w http.ResponseWriter, r *http.Request, runID string) {
		writeRunComplete(t, w, proto.RunComplete{
			SessionID: "s",
			RunID:     runID,
			MessageID: "m",
			Text:      "answer",
		})
		<-r.Context().Done()
	})
	require.NoError(t, runAgainst(t, c))
}
