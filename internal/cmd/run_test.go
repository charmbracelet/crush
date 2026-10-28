package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// runServerOpts describes the server runNonInteractive talks to.
type runServerOpts struct {
	// holdSend keeps POST /agent from answering, so the interrupt lands
	// while the prompt is still in flight.
	holdSend bool
	// complete reports the run finished as soon as it is accepted.
	// Otherwise the run never ends on its own, like a run the server
	// keeps going because another client still holds the workspace.
	complete bool
}

// runServer records what the fake server saw.
type runServer struct {
	// sent is closed once POST /agent has the prompt: after the server
	// answered that it accepted the run, or, with holdSend, while the
	// request is still open.
	sent chan struct{}

	mu        sync.Mutex
	cancelled []string
}

func (s *runServer) cancelledSessions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.cancelled...)
}

func newRunServer(t *testing.T, opts runServerOpts) (*client.Client, *runServer) {
	t.Helper()
	rs := &runServer{sent: make(chan struct{})}
	accepted := make(chan string, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/workspaces/{id}/agent", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(proto.AgentInfo{IsReady: true})
	})
	mux.HandleFunc("POST /v1/workspaces/{id}/agent/update", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /v1/workspaces/{id}/sessions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(proto.Session{ID: "sess-1"})
	})
	mux.HandleFunc("GET /v1/workspaces/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case runID := <-accepted:
			if opts.complete {
				inner, _ := json.Marshal(pubsub.Event[proto.RunComplete]{
					Type:    pubsub.UpdatedEvent,
					Payload: proto.RunComplete{SessionID: "sess-1", RunID: runID},
				})
				data, _ := json.Marshal(pubsub.Payload{Type: pubsub.PayloadTypeRunComplete, Payload: inner})
				fmt.Fprintf(w, "data: %s\n\n", data)
				w.(http.Flusher).Flush()
			}
		case <-r.Context().Done():
		}
		<-r.Context().Done()
	})
	mux.HandleFunc("POST /v1/workspaces/{id}/agent", func(w http.ResponseWriter, r *http.Request) {
		var msg proto.AgentMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if opts.holdSend {
			close(rs.sent)
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusAccepted)
		w.(http.Flusher).Flush()
		accepted <- msg.RunID
		close(rs.sent)
	})
	mux.HandleFunc("POST /v1/workspaces/{id}/agent/sessions/{sid}/cancel", func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.cancelled = append(rs.cancelled, r.PathValue("sid"))
		rs.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	return c, rs
}

func runWorkspace() *proto.Workspace {
	return &proto.Workspace{ID: "ws-1", Config: &config.Config{Options: &config.Options{}}}
}

// TestRunNonInteractive_InterruptCancelsServerRun is the regression test
// for Ctrl+C on `crush run` in client/server mode leaving its agent run
// going on the server. The workspace owns the run, so while another
// client holds the workspace only the cancel endpoint can stop it, and
// it has to be sent before the process exits.
func TestRunNonInteractive_InterruptCancelsServerRun(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		holdSend bool
	}{
		{name: "after the server accepted the prompt"},
		{name: "while the prompt is in flight", holdSend: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c, srv := newRunServer(t, runServerOpts{holdSend: tc.holdSend})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			errc := make(chan error, 1)
			go func() {
				errc <- runNonInteractive(ctx, c, runWorkspace(), "hi", "", "", "", true, "", false)
			}()

			select {
			case <-srv.sent:
			case <-time.After(10 * time.Second):
				t.Fatal("the prompt never reached the server")
			}
			cancel()

			// The returned error is not checked: the interrupt also ends
			// the event stream, which the run loop may notice first.
			select {
			case <-errc:
			case <-time.After(10 * time.Second):
				t.Fatal("runNonInteractive did not return after the interrupt")
			}
			require.Equal(t, []string{"sess-1"}, srv.cancelledSessions(),
				"an interrupted run must ask the server to cancel its session's run before returning")
		})
	}
}

// TestRunNonInteractive_CompletedRunIsNotCancelled makes sure only an
// interrupt cancels: the session can be shared with another client, whose
// prompts must survive a `crush run` that finished normally.
func TestRunNonInteractive_CompletedRunIsNotCancelled(t *testing.T) {
	t.Parallel()

	c, srv := newRunServer(t, runServerOpts{complete: true})
	err := runNonInteractive(t.Context(), c, runWorkspace(), "hi", "", "", "", true, "", false)
	require.NoError(t, err)
	require.Empty(t, srv.cancelledSessions())
}
