package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

const lspHostJSON = `{"protocol":1,"name":"joy","version":"0.9.0","pid":43549,` +
	`"address":"unix:///run/joy-501/lsp-3f9c2a1b7e4d5a60.sock",` +
	`"servers":[{"name":"gopls","command":"gopls"},{"name":"rust-analyzer","command":"rust-analyzer"}]}`

func testLSPHost() *proto.LSPHost {
	return &proto.LSPHost{
		Protocol: 1,
		Name:     "joy",
		Version:  "0.9.0",
		PID:      43549,
		Address:  "unix:///run/joy-501/lsp-3f9c2a1b7e4d5a60.sock",
		Servers: []proto.LSPHostServer{
			{Name: "gopls", Command: "gopls"},
			{Name: "rust-analyzer", Command: "rust-analyzer"},
		},
	}
}

func TestSubscribeEventsDecodesLSPHost(t *testing.T) {
	t.Parallel()

	// The exact wire shapes: an envelope whose payload is a pubsub event.
	updated := `{"type":"lsp_host","payload":{"type":"updated","payload":` +
		`{"workspace_id":"ws1","client_id":"c1","lsp_host":` + lspHostJSON + `}}}`
	const deleted = `{"type":"lsp_host","payload":{"type":"deleted","payload":` +
		`{"workspace_id":"ws1","lsp_host":null}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: %s\n\n", updated, deleted)
		w.(http.Flusher).Flush()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := captureClient(t, srv).SubscribeEvents(ctx, "ws1")
	require.NoError(t, err)

	next := func() pubsub.Event[proto.LSPHostState] {
		select {
		case ev := <-events:
			e, ok := ev.(pubsub.Event[proto.LSPHostState])
			require.True(t, ok, "got %T", ev)
			return e
		case <-time.After(5 * time.Second):
			require.Fail(t, "timed out waiting for the LSP host event")
			return pubsub.Event[proto.LSPHostState]{}
		}
	}

	e := next()
	require.Equal(t, pubsub.UpdatedEvent, e.Type)
	require.Equal(t, proto.LSPHostState{WorkspaceID: "ws1", ClientID: "c1", LSPHost: testLSPHost()}, e.Payload)

	e = next()
	require.Equal(t, pubsub.DeletedEvent, e.Type)
	require.Equal(t, proto.LSPHostState{WorkspaceID: "ws1"}, e.Payload)
}

func TestSetLSPHost(t *testing.T) {
	t.Parallel()

	var (
		gotPath  string
		gotQuery string
		gotBody  string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotPath, gotQuery, gotBody = r.Method+" "+r.URL.Path, r.URL.Query().Get("client_id"), string(body)
		var req proto.LSPHostRequest
		require.NoError(t, json.Unmarshal(body, &req))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(proto.LSPHostState{WorkspaceID: "ws1", ClientID: gotQuery, LSPHost: req.LSPHost})
	}))
	defer srv.Close()
	c := captureClient(t, srv)

	st, err := c.SetLSPHost(t.Context(), "ws1", testLSPHost())
	require.NoError(t, err)
	require.Equal(t, "POST /v1/workspaces/ws1/lsp-host", gotPath)
	require.Equal(t, c.ClientID(), gotQuery)
	require.JSONEq(t, `{"lsp_host":`+lspHostJSON+`}`, gotBody)
	require.Equal(t, &proto.LSPHostState{WorkspaceID: "ws1", ClientID: c.ClientID(), LSPHost: testLSPHost()}, st)

	st, err = c.SetLSPHost(t.Context(), "ws1", nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"lsp_host":null}`, gotBody)
	require.Nil(t, st.LSPHost)
}

func TestSetLSPHostReportsServerMessage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(proto.Error{Message: "client holds no claim on the workspace"})
	}))
	defer srv.Close()

	_, err := captureClient(t, srv).SetLSPHost(t.Context(), "ws1", testLSPHost())
	require.ErrorContains(t, err, "no claim")
}

func TestGetLSPHost(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/workspaces/ws1/lsp-host":
			_, _ = io.WriteString(w, `{"workspace_id":"ws1","client_id":"c1","lsp_host":`+lspHostJSON+`}`)
		case "/v1/workspaces/ws2/lsp-host":
			_, _ = io.WriteString(w, `{"workspace_id":"ws2","lsp_host":null}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := captureClient(t, srv)

	st, err := c.GetLSPHost(t.Context(), "ws1")
	require.NoError(t, err)
	require.Equal(t, &proto.LSPHostState{WorkspaceID: "ws1", ClientID: "c1", LSPHost: testLSPHost()}, st)

	st, err = c.GetLSPHost(t.Context(), "ws2")
	require.NoError(t, err)
	require.Nil(t, st.LSPHost)

	_, err = c.GetLSPHost(t.Context(), "missing")
	require.ErrorIs(t, err, ErrNotFound)
}
