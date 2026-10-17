package workspace

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestClientWorkspace_StaleRefreshDuringRecovery(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))
	oldClose, refreshRelease, headersRelease := make(chan struct{}), make(chan struct{}), make(chan struct{})
	refreshEntered, newEntered := make(chan struct{}), make(chan struct{})
	closeOld := sync.OnceFunc(func() { close(oldClose) })
	releaseRefresh := sync.OnceFunc(func() { close(refreshRelease) })
	releaseHeaders := sync.OnceFunc(func() { close(headersRelease) })
	var oldStreams atomic.Int32
	emit := func(w http.ResponseWriter, sequence uint64) {
		payload, err := json.Marshal(pubsub.Event[proto.RunComplete]{Type: pubsub.UpdatedEvent, Payload: proto.RunComplete{SessionID: "S", Sequence: sequence}})
		require.NoError(t, err)
		envelope, err := json.Marshal(pubsub.Payload{Type: pubsub.PayloadTypeRunComplete, Payload: payload})
		require.NoError(t, err)
		fmt.Fprintf(w, "data: %s\n\n", envelope)
		w.(http.Flusher).Flush()
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/W1":
			close(refreshEntered)
			select {
			case <-refreshRelease:
			case <-r.Context().Done():
				return
			}
			json.NewEncoder(w).Encode(proto.Workspace{ID: "W1", Path: "/workspace"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces":
			json.NewEncoder(w).Encode(proto.Workspace{ID: "W2", Path: "/workspace"})
		case r.URL.Path == "/v1/workspaces/W1/events":
			if oldStreams.Add(1) > 1 {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			emit(w, 100)
			select {
			case <-oldClose:
			case <-r.Context().Done():
			}
		case r.URL.Path == "/v1/workspaces/W2/events":
			close(newEntered)
			select {
			case <-headersRelease:
			case <-r.Context().Done():
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			emit(w, 1)
			<-r.Context().Done()
		case strings.Contains(r.URL.Path, "/config"):
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	defer closeOld()
	defer releaseRefresh()
	defer releaseHeaders()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	ws := NewClientWorkspace(c, proto.Workspace{ID: "W1", Path: "/workspace"})
	defer ws.subCancel()
	events := make(chan tea.Msg, 20)
	done := make(chan struct{})
	go func() { ws.runSubscription(func(m tea.Msg) { events <- m }); close(done) }()
	wait := func(ch <-chan struct{}, what string) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(3 * time.Second):
			t.Fatalf("gate timed out: %s", what)
		}
	}
	next := func() tea.Msg {
		t.Helper()
		select {
		case m := <-events:
			return m
		case <-time.After(3 * time.Second):
			t.Fatal("event timeout")
			return nil
		}
	}
	old := next().(pubsub.Event[notify.RunComplete])
	require.Equal(t, uint64(100), old.Payload.Sequence)
	refreshDone := make(chan struct{})
	go func() { defer close(refreshDone); require.NoError(t, ws.SetCompactMode(config.ScopeWorkspace, true)) }()
	wait(refreshEntered, "W1 refresh GET")
	closeOld()
	wait(newEntered, "W2 subscription request before headers")
	require.Equal(t, "W2", ws.workspaceID(), "recovery really adopted W2")
	releaseRefresh()
	wait(refreshDone, "old refresh completes")
	releaseHeaders()
	var recovered *ConnectionEvent
	for {
		ev := next()
		if connection, ok := ev.(ConnectionEvent); ok && connection.State == ConnectionRecovered {
			recovered = &connection
		}
		if completion, ok := ev.(pubsub.Event[notify.RunComplete]); ok {
			require.Equal(t, uint64(1), completion.Payload.Sequence)
			require.NotNil(t, recovered, "boundary must precede new stream completion")
			break
		}
	}
	ws.subCancel()
	wait(done, "subscription exits")
	if got := ws.workspaceID(); got != "W2" {
		t.Errorf("stale refresh restored old workspace identity: got %s, want W2", got)
	}
	require.True(t, recovered.WorkspaceRecreated, "new instance must reset the completion comparison scope")
}
