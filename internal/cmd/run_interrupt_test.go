package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// interruptRun configures runInLoop.
type interruptRun struct {
	continueID string // As `run -s <id>`; empty creates session "own".
	complete   bool   // Finish the run normally once it is in its loop.
	oldServer  bool   // Server không có endpoint huỷ theo RunID (404).
	onCancel   func(r *http.Request, target string)
}

// runInLoop starts runNonInteractive for session "own" against a fake
// server and returns once the run is in its event loop: a prompt from
// another session is pushed after the send, and the run looking that
// session up proves it got there.
func runInLoop(t *testing.T, o interruptRun) (interrupt func(), done <-chan error) {
	t.Helper()
	sent := make(chan struct{})
	var runID string // Written before sent is closed, read after.
	inLoop := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/v1/workspaces/ws")
		switch {
		case p == "/agent" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(proto.AgentInfo{IsReady: true})
		case p == "/agent" && r.Method == http.MethodPost:
			var msg proto.AgentMessage
			_ = json.NewDecoder(r.Body).Decode(&msg)
			runID = msg.RunID
			close(sent)
		case p == "/sessions/own":
			_ = json.NewEncoder(w).Encode(proto.Session{ID: "own"})
		case p == "/sessions/own/messages":
			_ = json.NewEncoder(w).Encode([]proto.Message{})
		case p == "/sessions/other":
			_ = json.NewEncoder(w).Encode(proto.Session{ID: "other"})
			inLoop <- struct{}{}
		case p == "/sessions" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(proto.Session{ID: "own"})
		case p == "/events":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-sent
			ev, _ := json.Marshal(pubsub.Event[proto.PermissionRequest]{
				Type: pubsub.CreatedEvent, Payload: proto.PermissionRequest{SessionID: "other", ToolName: "bash"},
			})
			env, _ := json.Marshal(pubsub.Payload{Type: pubsub.PayloadTypePermissionRequest, Payload: ev})
			fmt.Fprintf(w, "data: %s\n\n", env)
			if o.complete {
				ev, _ = json.Marshal(pubsub.Event[proto.RunComplete]{
					Type: pubsub.UpdatedEvent, Payload: proto.RunComplete{SessionID: "own", RunID: runID, Text: "done"},
				})
				env, _ = json.Marshal(pubsub.Payload{Type: pubsub.PayloadTypeRunComplete, Payload: ev})
				fmt.Fprintf(w, "data: %s\n\n", env)
			}
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case strings.HasPrefix(p, "/agent/sessions/") && strings.Contains(p, "/runs/") && strings.HasSuffix(p, "/cancel"):
			if o.oldServer {
				http.NotFound(w, r)
				return
			}
			// Ghi "<session>/run=<đúng RunID đã gửi?>" để test so được.
			sid, rest, _ := strings.Cut(strings.TrimPrefix(p, "/agent/sessions/"), "/runs/")
			o.onCancel(r, fmt.Sprintf("%s/run=%t", sid, strings.TrimSuffix(rest, "/cancel") == runID))
		case strings.HasPrefix(p, "/agent/sessions/") && strings.HasSuffix(p, "/cancel"):
			o.onCancel(r, strings.TrimSuffix(strings.TrimPrefix(p, "/agent/sessions/"), "/cancel"))
		}
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient("", "tcp", u.Host)
	require.NoError(t, err)

	ws := &proto.Workspace{ID: "ws", Config: &config.Config{Options: &config.Options{}}}
	ctx, cancel := context.WithCancel(t.Context())
	ch := make(chan error, 1)
	go func() {
		ch <- runNonInteractive(ctx, c, ws, io.Discard, nil, "SLOW", "", "", "", true, o.continueID, false)
	}()
	select {
	case <-inLoop:
	case <-time.After(10 * time.Second):
		t.Fatal("run never reached its event loop")
	}
	return cancel, ch
}

// TestRunInterruptCancelsOwnRun pins what Ctrl+C on `run` means in
// client/server mode, where the server keeps a run going after its client
// leaves: the run cancels its own turn by RunID, on a created session and
// on a continued one (-s/-C) alike, so other turns and queued prompts of
// the session are untouched. An old server without the per-run cancel:
// a created session holds only this run's work and is cancelled whole; a
// continued session is left alone and the run says it keeps going. A run
// that completes cancels nothing.
func TestRunInterruptCancelsOwnRun(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		run         interruptRun
		interrupt   bool
		wantCancels []string
		wantErr     string
	}{
		"created session":   {interrupt: true, wantCancels: []string{"own/run=true"}, wantErr: context.Canceled.Error()},
		"continued session": {run: interruptRun{continueID: "own"}, interrupt: true, wantCancels: []string{"own/run=true"}, wantErr: context.Canceled.Error()},
		"old server, created session": {
			run: interruptRun{oldServer: true}, interrupt: true,
			wantCancels: []string{"own"}, wantErr: context.Canceled.Error(),
		},
		"old server, continued session": {
			run: interruptRun{continueID: "own", oldServer: true}, interrupt: true,
			wantErr: "stopped waiting; the run keeps going on session own (this server cannot cancel a single run)",
		},
		"completed run": {run: interruptRun{complete: true}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var (
				mu      sync.Mutex
				cancels []string
			)
			tc.run.onCancel = func(_ *http.Request, id string) {
				mu.Lock()
				cancels = append(cancels, id)
				mu.Unlock()
			}
			interrupt, done := runInLoop(t, tc.run)
			if tc.interrupt {
				interrupt()
			}
			err := <-done
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr)
			}

			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, tc.wantCancels, cancels)
		})
	}
}

// TestRunInterruptCancelIsBounded: a hung server must not hang Ctrl+C. The
// cancel request gives up after cancelRunTimeout and says the run may still
// be going.
func TestRunInterruptCancelIsBounded(t *testing.T) {
	// Not parallel: it shortens package-level knobs.
	var stderr bytes.Buffer
	prevTimeout, prevStderr := cancelRunTimeout, runStderr
	cancelRunTimeout, runStderr = 100*time.Millisecond, &stderr
	t.Cleanup(func() { cancelRunTimeout, runStderr = prevTimeout, prevStderr })

	release := make(chan struct{})
	interrupt, done := runInLoop(t, interruptRun{onCancel: func(r *http.Request, _ string) {
		select { // A server that never answers the cancel.
		case <-r.Context().Done():
		case <-release:
		}
	}})
	t.Cleanup(func() { close(release) }) // Runs before srv.Close.

	interrupt()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("interrupt hung on the cancel request")
	}
	require.Equal(t, "cancel request timed out; the run may still be going on session own\n", stderr.String())
}

// TestRunQueuedBehindOtherTurnLeavesItsPromptsAlone: `run -s S` can queue
// behind a TUI turn in S. That turn's permission requests and questions are
// not the run's: approving one would run the user's tool unasked, failing on
// one would cancel S and kill the user's turn. While the run's prompt is
// still queued (or the queue cannot be read) the run keeps waiting and says
// so; once its prompt is the active turn it acts as before.
func TestRunQueuedBehindOtherTurnLeavesItsPromptsAlone(t *testing.T) {
	// Not parallel: swaps runStderr.
	cases := map[string]struct {
		queue                   func(w http.ResponseWriter)
		wantErr                 string
		wantGrants, wantCancels int
		wantNotices             int
	}{
		"other turn active": {
			queue:       func(w http.ResponseWriter) { _ = json.NewEncoder(w).Encode([]string{"tui follow-up", "PROMPT"}) },
			wantNotices: 2,
		},
		"queue unreadable": {
			queue:       func(w http.ResponseWriter) { http.Error(w, "boom", http.StatusInternalServerError) },
			wantNotices: 2,
		},
		"own turn active": {
			queue:   func(w http.ResponseWriter) { _ = json.NewEncoder(w).Encode([]string{"tui follow-up"}) },
			wantErr: "the agent asked a question", wantGrants: 1, wantCancels: 1,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			prev := runStderr
			runStderr = &stderr
			t.Cleanup(func() { runStderr = prev })

			var (
				mu                          sync.Mutex
				grants, cancels, runCancels int
				runID                       string // Written before sent is closed, read after.
			)
			sent := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p := strings.TrimPrefix(r.URL.Path, "/v1/workspaces/ws")
				switch {
				case p == "/agent" && r.Method == http.MethodGet:
					_ = json.NewEncoder(w).Encode(proto.AgentInfo{IsReady: true})
				case p == "/agent" && r.Method == http.MethodPost:
					var msg proto.AgentMessage
					_ = json.NewDecoder(r.Body).Decode(&msg)
					runID = msg.RunID
					close(sent)
				case p == "/sessions/S":
					_ = json.NewEncoder(w).Encode(proto.Session{ID: "S"})
				case p == "/sessions/S/messages":
					_ = json.NewEncoder(w).Encode([]proto.Message{})
				case p == "/agent/sessions/S/prompts/list":
					tc.queue(w)
				case p == "/permissions/grant":
					mu.Lock()
					grants++
					mu.Unlock()
					_ = json.NewEncoder(w).Encode(proto.PermissionGrantResponse{})
				case p == "/agent/sessions/S/cancel":
					mu.Lock()
					cancels++
					mu.Unlock()
				case p == "/agent/sessions/S/runs/"+runID+"/cancel":
					mu.Lock()
					runCancels++
					mu.Unlock()
				case p == "/events":
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
					<-sent
					emit := func(typ pubsub.PayloadType, ev any) {
						raw, _ := json.Marshal(ev)
						env, _ := json.Marshal(pubsub.Payload{Type: typ, Payload: raw})
						fmt.Fprintf(w, "data: %s\n\n", env)
					}
					emit(pubsub.PayloadTypePermissionRequest, pubsub.Event[proto.PermissionRequest]{
						Type: pubsub.CreatedEvent, Payload: proto.PermissionRequest{ID: "p1", SessionID: "S", ToolName: "bash"},
					})
					emit(pubsub.PayloadTypeQuestionRequest, pubsub.Event[proto.QuestionRequest]{
						Type: pubsub.CreatedEvent, Payload: proto.QuestionRequest{ID: "q1", SessionID: "S"},
					})
					emit(pubsub.PayloadTypeRunComplete, pubsub.Event[proto.RunComplete]{
						Type: pubsub.UpdatedEvent, Payload: proto.RunComplete{SessionID: "S", RunID: runID, Text: "done"},
					})
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}
			}))
			t.Cleanup(srv.Close)
			u, err := url.Parse(srv.URL)
			require.NoError(t, err)
			c, err := client.NewClient("", "tcp", u.Host)
			require.NoError(t, err)
			ws := &proto.Workspace{ID: "ws", Config: &config.Config{Options: &config.Options{}}}

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			err = runNonInteractive(ctx, c, ws, io.Discard, nil, "PROMPT", "", "", "", true, "S", false)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}

			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, tc.wantGrants, grants, "grants")
			require.Equal(t, tc.wantCancels, runCancels, "cancels of this run's own turn")
			require.Zero(t, cancels, "never cancel the whole session S: other turns live there")
			require.Equal(t, tc.wantNotices, strings.Count(stderr.String(), "another turn"), stderr.String())
		})
	}
}

// TestRunEventStreamClosedEarlyFails: the server closing the event stream
// before this run's RunComplete (crash, restart) must fail the run, so
// --json reports an error and exit 1 instead of a truncated success.
func TestRunEventStreamClosedEarlyFails(t *testing.T) {
	t.Parallel()

	sent := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/v1/workspaces/ws")
		switch {
		case p == "/agent" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(proto.AgentInfo{IsReady: true})
		case p == "/agent" && r.Method == http.MethodPost:
			close(sent)
		case p == "/sessions" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(proto.Session{ID: "own"})
		case p == "/events":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-sent // Then end the stream with no RunComplete.
		}
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient("", "tcp", u.Host)
	require.NoError(t, err)
	ws := &proto.Workspace{ID: "ws", Config: &config.Config{Options: &config.Options{}}}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err = runNonInteractive(ctx, c, ws, io.Discard, nil, "P", "", "", "", true, "", false)
	require.EqualError(t, err, "event stream closed before the run completed")
}
