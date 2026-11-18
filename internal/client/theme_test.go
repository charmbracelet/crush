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

func TestSubscribeEventsDecodesHostTheme(t *testing.T) {
	t.Parallel()

	// The exact wire shape: an envelope whose payload is a pubsub event.
	const data = `{"type":"host_theme","payload":{"type":"updated","payload":` +
		`{"workspace_id":"ws1","theme":{"name":"charmtone-dark","source":"superjoy",` +
		`"palette":{"primary":"#6B50FF"},"syntax":{"keyword":"#00A4FF"},"transparent":false}}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := captureClient(t, srv).SubscribeEvents(ctx, "ws1")
	require.NoError(t, err)

	select {
	case ev := <-events:
		e, ok := ev.(pubsub.Event[proto.HostThemeState])
		require.True(t, ok, "got %T", ev)
		require.Equal(t, pubsub.UpdatedEvent, e.Type)
		require.Equal(t, "ws1", e.Payload.WorkspaceID)
		require.NotNil(t, e.Payload.Theme)
		require.Equal(t, "charmtone-dark", e.Payload.Theme.Name)
		require.Equal(t, "#6B50FF", e.Payload.Theme.Palette["primary"])
		require.Equal(t, "#00A4FF", e.Payload.Theme.Syntax["keyword"])
		require.NotNil(t, e.Payload.Theme.Transparent)
		require.False(t, *e.Payload.Theme.Transparent)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for the host theme event")
	}
}

func TestSetHostTheme(t *testing.T) {
	t.Parallel()

	var (
		gotPath  string
		gotQuery string
		gotBody  string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotPath, gotQuery, gotBody = r.Method+" "+r.URL.Path, r.URL.Query().Get("client_id"), string(body)
		var req proto.HostThemeRequest
		require.NoError(t, json.Unmarshal(body, &req))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(proto.HostThemeState{WorkspaceID: "ws1", Theme: req.Theme})
	}))
	defer srv.Close()
	c := captureClient(t, srv)

	theme := &proto.HostTheme{Name: "t", Palette: map[string]string{"primary": "#6B50FF"}}
	st, err := c.SetHostTheme(t.Context(), "ws1", theme)
	require.NoError(t, err)
	require.Equal(t, "POST /v1/workspaces/ws1/theme", gotPath)
	require.Equal(t, c.ClientID(), gotQuery)
	require.JSONEq(t, `{"theme":{"name":"t","palette":{"primary":"#6B50FF"}}}`, gotBody)
	require.Equal(t, "ws1", st.WorkspaceID)
	require.True(t, theme.Equal(st.Theme))

	st, err = c.SetHostTheme(t.Context(), "ws1", nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"theme":null}`, gotBody)
	require.Nil(t, st.Theme)
}

func TestSetHostThemeReportsServerMessage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(proto.Error{Message: `invalid host theme: palette.primary: invalid color "x"`})
	}))
	defer srv.Close()

	_, err := captureClient(t, srv).SetHostTheme(t.Context(), "ws1", &proto.HostTheme{})
	require.ErrorContains(t, err, "palette.primary")
}

func TestGetHostTheme(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws1/theme" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, `{"workspace_id":"ws1","theme":{"name":"t","base":"gruvbox-dark"}}`)
	}))
	defer srv.Close()
	c := captureClient(t, srv)

	st, err := c.GetHostTheme(t.Context(), "ws1")
	require.NoError(t, err)
	require.Equal(t, "gruvbox-dark", st.Theme.Base)

	_, err = c.GetHostTheme(t.Context(), "missing")
	require.ErrorIs(t, err, ErrNotFound)
}
