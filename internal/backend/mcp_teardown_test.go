package backend

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// TestTeardown_KeepsOtherWorkspacesMCP guards against a workspace teardown
// closing every MCP session in the process and shutting the shared MCP
// event broker down, and against a reopened workspace leaking the session
// it replaces while another workspace keeps the MCP clients referenced.
func TestTeardown_KeepsOtherWorkspacesMCP(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	const name = "teardown-probe"
	httpSrv := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcp.NewServer(&mcp.Implementation{Name: name}, nil)
	}, nil))
	t.Cleanup(func() { httpSrv.CloseClientConnections(); httpSrv.Close() })
	b := New(t.Context(), nil, func() {})
	b.SetCreateGrace(time.Minute)
	t.Cleanup(func() { drainBackend(t, b) })

	// Only workspace A uses the MCP server.
	dirA, dataA := t.TempDir(), t.TempDir()
	cfg := fmt.Sprintf(`{"mcp": {%q: {"type": "http", "url": %q}}}`, name, httpSrv.URL)
	require.NoError(t, os.WriteFile(filepath.Join(dirA, "crush.json"), []byte(cfg), 0o644))
	connected := func(prev *mcptools.ClientSession) (got *mcptools.ClientSession) {
		t.Helper()
		require.Eventually(t, func() bool {
			info, ok := mcptools.GetState(name)
			got = info.Client
			return ok && info.State == mcptools.StateConnected && got != nil && got != prev
		}, 10*time.Second, 20*time.Millisecond, "MCP server never connected")
		return got
	}

	cidA, cidB := newClientID(t), newClientID(t)
	_, _, err := b.CreateWorkspace(protoWS(dirA, dataA, cidA))
	require.NoError(t, err)
	_, _, err = b.CreateWorkspace(protoWS(t.TempDir(), t.TempDir(), cidB))
	require.NoError(t, err)
	first := connected(nil)

	require.NoError(t, b.RetireClient(cidB))
	require.NoError(t, first.Ping(t.Context(), nil), "another workspace's teardown closed the MCP session")

	// While B holds the clients, A is reopened and must close the session
	// it replaces.
	cidB = newClientID(t)
	_, _, err = b.CreateWorkspace(protoWS(t.TempDir(), t.TempDir(), cidB))
	require.NoError(t, err)
	require.NoError(t, b.RetireClient(cidA))
	cidA = newClientID(t)
	_, _, err = b.CreateWorkspace(protoWS(dirA, dataA, cidA))
	require.NoError(t, err)
	second := connected(first)
	require.Eventually(t, func() bool { return first.Ping(t.Context(), nil) != nil },
		5*time.Second, 20*time.Millisecond, "the reopened workspace leaked its replaced MCP session")
	require.NoError(t, second.Ping(t.Context(), nil))

	// The last teardown must not shut the shared event broker down for a
	// workspace opened later.
	require.NoError(t, b.RetireClient(cidA))
	require.NoError(t, b.RetireClient(cidB))
	ws, _, err := b.CreateWorkspace(protoWS(t.TempDir(), t.TempDir(), newClientID(t)))
	require.NoError(t, err)
	events, err := b.SubscribeEvents(t.Context(), ws.ID)
	require.NoError(t, err)
	requireMCPDisabledEvent(t, ws, events, name)
}

// requireMCPDisabledEvent disables the named MCP server until its state
// change reaches ch; the app subscribes its event fan-in asynchronously.
func requireMCPDisabledEvent(t *testing.T, ws *Workspace, ch <-chan pubsub.Event[tea.Msg], name string) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		require.NoError(t, mcptools.DisableSingle(ws.Cfg, name))
		select {
		case ev, ok := <-ch:
			require.True(t, ok, "event channel was closed")
			if e, ok := ev.Payload.(pubsub.Event[mcptools.Event]); ok &&
				e.Payload.Name == name && e.Payload.State == mcptools.StateDisabled {
				return
			}
		case <-time.After(50 * time.Millisecond):
		case <-timeout:
			t.Fatal("timed out waiting for the MCP state change")
		}
	}
}
