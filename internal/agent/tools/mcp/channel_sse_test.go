package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// newStreamableChannelServer starts a minimal streamable-HTTP MCP server that
// answers the legacy initialize handshake, advertises the claude/channel
// capability, and serves a standalone SSE GET stream. The returned record
// channel reports what the client actually did on the wire.
type streamableRecord struct {
	sawGET      chan struct{}
	sawDiscover chan struct{}
}

func newStreamableChannelServer(t *testing.T, name string, doorbell func(w http.ResponseWriter)) (*httptest.Server, *streamableRecord) {
	t.Helper()
	rec := &streamableRecord{sawGET: make(chan struct{}, 4), sawDiscover: make(chan struct{}, 4)}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			select {
			case rec.sawGET <- struct{}{}:
			default:
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			if doorbell != nil {
				doorbell(w)
			}
			<-r.Context().Done()
		case http.MethodPost:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var req struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			require.NoError(t, json.Unmarshal(body, &req))
			id := string(bytes.Trim(req.ID, "null"))
			if id == "" {
				id = "1"
			}
			if req.Method == "initialize" {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Mcp-Session-Id", "test-session")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + id + `,"result":{"protocolVersion":"2025-06-18","capabilities":{"experimental":{"claude/channel":{}}},"serverInfo":{"name":"t","version":"0"}}}`))
				return
			}
			if req.Method == "notifications/initialized" {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			// Anything else (e.g. server/discover) is refused so the SDK
			// falls back to the legacy initialize handshake.
			select {
			case rec.sawDiscover <- struct{}{}:
			default:
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + id + `,"error":{"code":-32601,"message":"method not found"}}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, rec
}

// TestChannelStreamableStandaloneSSEOpens is the regression test for the
// doorbell deadness: the go-sdk only opens the standalone SSE stream by
// type-asserting the connection from Transport.Connect to its internal type,
// so a transport wrapper that wraps the connection silently suppresses the
// stream and the session stays ping-alive but doorbell-deaf. The filter must
// live below the connection (see channel_sse.go), leaving the connection
// unwrapped so the stream opens and doorbells flow.
func TestChannelStreamableStandaloneSSEOpens(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	doorbell := func(w http.ResponseWriter) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("event: message\ndata: " +
			`{"jsonrpc":"2.0","method":"notifications/claude/channel","params":{"content":"knock knock"}}` +
			"\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	const name = "chan-sse"
	srv, rec := newStreamableChannelServer(t, name, doorbell)

	sub := broker.Subscribe(ctx)
	gate := newChannelGate()
	client := mcp.NewClient(&mcp.Implementation{Name: "crush", Version: "test"}, nil)
	transport := &channelTransport{
		inner: &mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"},
		name:  name,
		gate:  gate,
	}
	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err)
	defer session.Close()

	select {
	case <-rec.sawGET:
	case <-time.After(5 * time.Second):
		t.Fatal("client never opened the standalone SSE GET stream")
	}

	// Mirror createSession: resolve the gate after Connect and publish
	// anything that arrived during negotiation.
	for _, raw := range gate.resolve(true) {
		publishChannelMessage(ctx, name, raw)
	}

	for {
		select {
		case ev := <-sub:
			// The broker is package-global and other tests publish on it;
			// only our own doorbell completes the test.
			if ev.Payload.Type != EventChannelMessage || ev.Payload.Name != name {
				continue
			}
			require.Contains(t, ev.Payload.ChannelMessage, "knock knock")
			return
		case <-ctx.Done():
			t.Fatal("timed out waiting for doorbell over the standalone SSE stream")
		}
	}
}

// TestChannelSSEFilterLeavesNonChannelEventsForSDK verifies the filter is
// transparent for ordinary notifications: they pass through unchanged and
// the SDK dispatches them (here: a tools/list_changed-shaped notification
// would reach the session, and a ping request is answered normally).
func TestChannelSSEFilterLeavesNonChannelEventsForSDK(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const name = "chan-pass"
	ping := func(w http.ResponseWriter) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("event: message\ndata: " +
			`{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info","data":"hi"}}` +
			"\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	srv, _ := newStreamableChannelServer(t, name, ping)

	gate := newChannelGate()
	gotLogging := make(chan struct{}, 1)
	client := mcp.NewClient(&mcp.Implementation{Name: "crush", Version: "test"}, &mcp.ClientOptions{
		//nolint:staticcheck // SA1019: only non-channel notification the SDK dispatches without extra capability negotiation
		LoggingMessageHandler: func(context.Context, *mcp.LoggingMessageRequest) {
			gotLogging <- struct{}{}
		},
	})
	transport := &channelTransport{
		inner: &mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"},
		name:  name,
		gate:  gate,
	}
	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err)
	defer session.Close()
	gate.resolve(false) // closed: channel events would be dropped, others pass

	select {
	case <-gotLogging:
	case <-time.After(5 * time.Second):
		t.Fatal("non-channel notification did not reach the SDK handler")
	}
}

// TestEventFilterStripsDoorbellAndKeepsKeepalive pins the parser: a doorbell is
// removed from the stream the SDK sees, everything else passes through byte for
// byte, including comment keepalives.
func TestEventFilterStripsDoorbellAndKeepsKeepalive(t *testing.T) {
	t.Parallel()
	gate := newChannelGate()
	// Resolve closed so the stripped doorbell is dropped instead of
	// published to the package-global broker other tests subscribe to.
	gate.resolve(false)
	ctx := context.Background()
	body := io.NopCloser(bytes.NewReader([]byte(
		": ok\n\n" +
			"event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"other/notification\"}\n\n" +
			"event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/claude/channel\",\"params\":{\"content\":\"hi\"}}\n\n" +
			": ping\n\n",
	)))
	b := &channelSSEBody{
		ctx:    ctx,
		body:   body,
		filter: &channelSSEFilter{name: "chan-filter", gate: gate},
	}
	out, err := io.ReadAll(b)
	require.NoError(t, err)
	require.NotContains(t, string(out), "claude/channel", "doorbell must be stripped")
	require.Contains(t, string(out), "other/notification")
	require.Contains(t, string(out), ": ok")
}

// TestChannelDoorbellBufferedUntilGateResolves verifies undecided-gate
// buffering survives the HTTP-layer path: a doorbell arriving before the
// gate resolves is published once it opens.
func TestChannelDoorbellBufferedUntilGateResolves(t *testing.T) {
	t.Parallel()
	gate := newChannelGate()
	raw := json.RawMessage(`{"content":"buffered"}`)
	require.Nil(t, gate.accept(raw), "undecided gate must buffer")
	require.Empty(t, gate.resolve(false))
	require.Nil(t, gate.accept(raw), "resolved-closed gate must drop")

	gate2 := newChannelGate()
	require.Nil(t, gate2.accept(raw))
	buffered := gate2.resolve(true)
	require.Len(t, buffered, 1)
	require.Equal(t, raw, buffered[0])
}
