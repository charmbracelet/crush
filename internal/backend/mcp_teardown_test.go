package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// TestTeardown_KeepsOtherWorkspacesMCP verifies that tearing a workspace
// down leaves the process-global MCP state other workspaces rely on alone.
// App.Shutdown used to close every MCP session in the process and shut the
// shared MCP event broker down for good, so the surviving workspace lost
// its MCP server, and it and every later workspace on the server lost all
// MCP events until the server restarted.
func TestTeardown_KeepsOtherWorkspacesMCP(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")

	// Workspace B uses an in-process HTTP MCP server; A and C have none.
	const name = "teardown-probe"
	server := mcp.NewServer(&mcp.Implementation{Name: name}, nil)
	httpSrv := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(func() {
		httpSrv.CloseClientConnections()
		httpSrv.Close()
	})
	dirA, dirB, dirC := t.TempDir(), t.TempDir(), t.TempDir()
	dataA, dataB, dataC := t.TempDir(), t.TempDir(), t.TempDir()
	cfgJSON, err := json.Marshal(map[string]any{
		"mcp": map[string]any{name: map[string]any{"type": "http", "url": httpSrv.URL}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dirB, "crush.json"), cfgJSON, 0o644))

	b := New(t.Context(), nil, func() {})
	b.SetCreateGrace(time.Minute)
	t.Cleanup(func() { drainBackend(t, b) })

	cidA, cidB := newClientID(t), newClientID(t)
	_, _, err = b.CreateWorkspace(protoWS(dirA, dataA, cidA))
	require.NoError(t, err)
	wsB, _, err := b.CreateWorkspace(protoWS(dirB, dataB, cidB))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		info, ok := mcptools.GetState(name)
		return ok && info.State == mcptools.StateConnected
	}, 10*time.Second, 20*time.Millisecond, "workspace B's MCP server never connected")
	info, _ := mcptools.GetState(name)
	session := info.Client
	require.NotNil(t, session)

	before := mcptools.SubscribeEvents(t.Context())
	eventsB, err := b.SubscribeEvents(t.Context(), wsB.ID)
	require.NoError(t, err)

	// Workspace A's only client exits, which tears A down.
	require.NoError(t, b.RetireClient(cidA))
	after := mcptools.SubscribeEvents(t.Context())

	t.Run("session stays open", func(t *testing.T) {
		require.NoError(t, session.Ping(t.Context(), nil),
			"workspace A's teardown closed workspace B's MCP session")
	})
	t.Run("subscription taken before teardown", func(t *testing.T) {
		requireMCPEvent(t, wsB.Cfg, before, name)
	})
	t.Run("subscription taken after teardown", func(t *testing.T) {
		requireMCPEvent(t, wsB.Cfg, after, name)
	})
	t.Run("workspace event stream", func(t *testing.T) {
		requireMCPEvent(t, wsB.Cfg, eventsB, name)
	})
	t.Run("workspace created after the last teardown", func(t *testing.T) {
		// B was the last workspace. A client that comes back within the
		// idle linger reuses this server and must still get MCP events.
		require.NoError(t, b.RetireClient(cidB))
		wsC, _, err := b.CreateWorkspace(protoWS(dirC, dataC, newClientID(t)))
		require.NoError(t, err)
		eventsC, err := b.SubscribeEvents(t.Context(), wsC.ID)
		require.NoError(t, err)
		requireMCPEvent(t, wsC.Cfg, eventsC, name)
	})
}

// TestTeardown_ReopenClosesReplacedMCPSession verifies that reopening a
// workspace while another one keeps the MCP clients referenced does not
// leak the first MCP session. A teardown that is not the last leaves its
// sessions open, so the reopened workspace's Initialize must close the
// session it replaces; otherwise no Close ever reaches it and its child
// lives until the server exits.
func TestTeardown_ReopenClosesReplacedMCPSession(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")

	// Workspace A uses an in-process HTTP MCP server; B has none.
	const name = "reopen-probe"
	server := mcp.NewServer(&mcp.Implementation{Name: name}, nil)
	httpSrv := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(func() {
		httpSrv.CloseClientConnections()
		httpSrv.Close()
	})
	dirA, dirB := t.TempDir(), t.TempDir()
	dataA, dataB := t.TempDir(), t.TempDir()
	cfgJSON, err := json.Marshal(map[string]any{
		"mcp": map[string]any{name: map[string]any{"type": "http", "url": httpSrv.URL}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dirA, "crush.json"), cfgJSON, 0o644))

	b := New(t.Context(), nil, func() {})
	b.SetCreateGrace(time.Minute)
	t.Cleanup(func() { drainBackend(t, b) })

	connected := func(prev *mcptools.ClientSession) *mcptools.ClientSession {
		t.Helper()
		var got *mcptools.ClientSession
		require.Eventually(t, func() bool {
			info, ok := mcptools.GetState(name)
			if !ok || info.State != mcptools.StateConnected || info.Client == prev {
				return false
			}
			got = info.Client
			return got != nil
		}, 10*time.Second, 20*time.Millisecond, "workspace A's MCP server never connected")
		return got
	}

	cidA, cidB := newClientID(t), newClientID(t)
	_, _, err = b.CreateWorkspace(protoWS(dirA, dataA, cidA))
	require.NoError(t, err)
	_, _, err = b.CreateWorkspace(protoWS(dirB, dataB, cidB))
	require.NoError(t, err)
	first := connected(nil)

	// A's client exits while B keeps the MCP clients referenced, then A
	// comes back with a new client and connects a new session.
	require.NoError(t, b.RetireClient(cidA))
	cidA2 := newClientID(t)
	_, _, err = b.CreateWorkspace(protoWS(dirA, dataA, cidA2))
	require.NoError(t, err)
	second := connected(first)

	require.Eventually(t, func() bool {
		return first.Ping(t.Context(), nil) != nil
	}, 5*time.Second, 20*time.Millisecond,
		"the reopened workspace left its replaced MCP session open")
	require.NoError(t, second.Ping(t.Context(), nil),
		"the reopened workspace's MCP session must stay open")

	// The last teardown still closes what is left.
	require.NoError(t, b.RetireClient(cidA2))
	require.NoError(t, b.RetireClient(cidB))
	require.Eventually(t, func() bool {
		return second.Ping(t.Context(), nil) != nil
	}, 5*time.Second, 20*time.Millisecond,
		"the last teardown left the MCP session open")
}

// requireMCPEvent disables the named MCP server, which publishes a state
// change, and requires that change to arrive on ch, either as a raw MCP
// event or wrapped in a workspace event. It publishes again until the
// change arrives, because an app subscribes its event fan-in
// asynchronously. It fails if ch closes or nothing arrives in time.
func requireMCPEvent[T any](t *testing.T, cfg *config.ConfigStore, ch <-chan pubsub.Event[T], name string) {
	t.Helper()
	retry := time.NewTicker(50 * time.Millisecond)
	defer retry.Stop()
	timeout := time.After(5 * time.Second)
	require.NoError(t, mcptools.DisableSingle(cfg, name))
	for {
		select {
		case ev, ok := <-ch:
			require.True(t, ok, "event channel was closed")
			var payload any = ev.Payload
			if wrapped, isWrapped := payload.(pubsub.Event[mcptools.Event]); isWrapped {
				payload = wrapped.Payload
			}
			if e, isMCP := payload.(mcptools.Event); isMCP &&
				e.Name == name && e.State == mcptools.StateDisabled {
				return
			}
		case <-retry.C:
			require.NoError(t, mcptools.DisableSingle(cfg, name))
		case <-timeout:
			t.Fatal("timed out waiting for the MCP state change")
		}
	}
}
