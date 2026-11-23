package backend

import (
	"fmt"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// testLSPHost returns a valid host registration named name.
// The workspaces of these tests use a host's language servers, as one a
// host started Crush for does. Set for the whole package, so the tests
// stay parallel; it only matters to the LSP host.
func init() { _ = os.Setenv(LSPHostEnv, "1") }

func testLSPHost(name string) *proto.LSPHost {
	return &proto.LSPHost{
		Protocol: 1,
		Name:     name,
		Version:  "1.2.3",
		PID:      4242,
		Address:  "unix:///run/" + name + "/lsp.sock",
		Servers: []proto.LSPHostServer{
			{Name: "gopls", Command: "gopls"},
			{Name: "rust-analyzer", Command: "rust-analyzer"},
		},
	}
}

// nextLSPHostEvent returns the next LSP host event published on the
// workspace broker, or fails after a short wait.
func nextLSPHostEvent(t *testing.T, events <-chan pubsub.Event[tea.Msg]) pubsub.Event[proto.LSPHostState] {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev := <-events:
			if e, ok := ev.Payload.(pubsub.Event[proto.LSPHostState]); ok {
				return e
			}
		case <-timeout:
			require.FailNow(t, "no LSP host event published")
		}
	}
}

// requireNoLSPHostEvent fails if an LSP host event is published within a
// short window.
func requireNoLSPHostEvent(t *testing.T, events <-chan pubsub.Event[tea.Msg]) {
	t.Helper()
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case ev := <-events:
			if e, ok := ev.Payload.(pubsub.Event[proto.LSPHostState]); ok {
				require.FailNow(t, "unexpected LSP host event", "%+v", e)
			}
		case <-timeout:
			return
		}
	}
}

// attachedClient returns the ID of a new client with an event stream on
// the workspace.
func attachedClient(t *testing.T, b *Backend, ws *Workspace) string {
	t.Helper()
	id := uuid.New().String()
	require.NoError(t, b.AttachClient(ws.ID, id))
	return id
}

func requireLSPHost(t *testing.T, b *Backend, ws *Workspace, clientID string, want *proto.LSPHost) {
	t.Helper()
	st, err := b.GetLSPHost(ws.ID)
	require.NoError(t, err)
	require.Equal(t, ws.ID, st.WorkspaceID)
	require.Equal(t, clientID, st.ClientID)
	require.True(t, want.Equal(st.LSPHost), "got %+v", st.LSPHost)
}

func TestSetLSPHost(t *testing.T) {
	t.Parallel()
	b, ws, _ := newPromptsTestBackend(t)
	events := ws.Events(t.Context())
	client := attachedClient(t, b, ws)

	requireLSPHost(t, b, ws, "", nil)

	host := testLSPHost("joy")
	st, err := b.SetLSPHost(ws.ID, client, host)
	require.NoError(t, err)
	require.Equal(t, proto.LSPHostState{WorkspaceID: ws.ID, ClientID: client, LSPHost: host}, st)
	ev := nextLSPHostEvent(t, events)
	require.Equal(t, pubsub.UpdatedEvent, ev.Type)
	require.Equal(t, proto.LSPHostState{WorkspaceID: ws.ID, ClientID: client, LSPHost: host}, ev.Payload)
	requireLSPHost(t, b, ws, client, host)

	// The stored host is a copy: the caller's later edits do not leak in.
	host.Servers[0].Command = "changed"
	requireLSPHost(t, b, ws, client, testLSPHost("joy"))

	// Registering the same host again publishes nothing.
	_, err = b.SetLSPHost(ws.ID, client, testLSPHost("joy"))
	require.NoError(t, err)
	requireNoLSPHostEvent(t, events)

	// A changed registration replaces the client's.
	changed := testLSPHost("joy")
	changed.Servers = changed.Servers[:1]
	_, err = b.SetLSPHost(ws.ID, client, changed)
	require.NoError(t, err)
	ev = nextLSPHostEvent(t, events)
	require.Equal(t, pubsub.UpdatedEvent, ev.Type)
	require.Len(t, ev.Payload.LSPHost.Servers, 1)

	st, err = b.SetLSPHost(ws.ID, client, nil)
	require.NoError(t, err)
	require.Equal(t, proto.LSPHostState{WorkspaceID: ws.ID, ClientID: client}, st)
	ev = nextLSPHostEvent(t, events)
	require.Equal(t, pubsub.DeletedEvent, ev.Type)
	require.Equal(t, proto.LSPHostState{WorkspaceID: ws.ID}, ev.Payload)
	requireLSPHost(t, b, ws, "", nil)

	// Clearing what is not there publishes nothing either.
	_, err = b.SetLSPHost(ws.ID, client, nil)
	require.NoError(t, err)
	requireNoLSPHostEvent(t, events)
}

func TestSetLSPHost_Errors(t *testing.T) {
	t.Parallel()
	b, ws, _ := newPromptsTestBackend(t)
	client := attachedClient(t, b, ws)

	_, err := b.SetLSPHost("missing", client, testLSPHost("joy"))
	require.ErrorIs(t, err, ErrWorkspaceNotFound)
	_, err = b.GetLSPHost("missing")
	require.ErrorIs(t, err, ErrWorkspaceNotFound)

	for _, id := range []string{"", "not-a-uuid"} {
		_, err = b.SetLSPHost(ws.ID, id, testLSPHost("joy"))
		require.ErrorIs(t, err, ErrInvalidClientID, id)
	}

	// A client without a claim on the workspace cannot register, nor
	// clear.
	stranger := uuid.New().String()
	_, err = b.SetLSPHost(ws.ID, stranger, testLSPHost("joy"))
	require.ErrorIs(t, err, ErrClientNoClaim)
	_, err = b.SetLSPHost(ws.ID, stranger, nil)
	require.ErrorIs(t, err, ErrClientNoClaim)

	invalid := map[string]func(h *proto.LSPHost){
		"protocol 0":       func(h *proto.LSPHost) { h.Protocol = 0 },
		"protocol 2":       func(h *proto.LSPHost) { h.Protocol = 2 },
		"no address":       func(h *proto.LSPHost) { h.Address = "" },
		"tcp address":      func(h *proto.LSPHost) { h.Address = "tcp://127.0.0.1:9000" },
		"bare path":        func(h *proto.LSPHost) { h.Address = "/run/joy/lsp.sock" },
		"relative address": func(h *proto.LSPHost) { h.Address = "unix://run/joy/lsp.sock" },
		"too many servers": func(h *proto.LSPHost) {
			h.Servers = nil
			for i := range proto.MaxLSPHostServers + 1 {
				h.Servers = append(h.Servers, proto.LSPHostServer{Name: fmt.Sprint("s", i), Command: fmt.Sprint("c", i)})
			}
		},
		"server without command": func(h *proto.LSPHost) { h.Servers[1].Command = "" },
		"server without name":    func(h *proto.LSPHost) { h.Servers[0].Name = "" },
	}
	for name, breakIt := range invalid {
		h := testLSPHost("joy")
		breakIt(h)
		_, err = b.SetLSPHost(ws.ID, client, h)
		require.ErrorIs(t, err, ErrInvalidLSPHost, name)
	}
	requireLSPHost(t, b, ws, "", nil)

	// The limits themselves are fine, and so is a host without servers.
	most := testLSPHost("joy")
	most.Servers = nil
	for i := range proto.MaxLSPHostServers {
		most.Servers = append(most.Servers, proto.LSPHostServer{Name: fmt.Sprint("s", i), Command: fmt.Sprint("c", i)})
	}
	_, err = b.SetLSPHost(ws.ID, client, most)
	require.NoError(t, err)
	none := testLSPHost("joy")
	none.Servers = nil
	_, err = b.SetLSPHost(ws.ID, client, none)
	require.NoError(t, err)
}

// TestSetLSPHost_HoldOnlyClient covers a client that created the workspace
// and has not attached its stream yet: it holds a claim, so it may
// register, and a duplicate create keeps its registration.
func TestSetLSPHost_HoldOnlyClient(t *testing.T) {
	t.Parallel()
	b, ws, _ := newPromptsTestBackend(t)
	b.createGrace = time.Hour
	client := uuid.New().String()
	b.registerClient(ws, client)

	_, err := b.SetLSPHost(ws.ID, client, testLSPHost("joy"))
	require.NoError(t, err)
	requireLSPHost(t, b, ws, client, testLSPHost("joy"))

	b.registerClient(ws, client)
	requireLSPHost(t, b, ws, client, testLSPHost("joy"))
}

// TestLSPHost_NewestRegistrationWins covers several hosts on one
// workspace: the newest registration is the workspace's host, and when it
// goes, however it goes, the one before it is again.
func TestLSPHost_NewestRegistrationWins(t *testing.T) {
	t.Parallel()
	b, ws, _ := newPromptsTestBackend(t)
	SetWorkspaceShutdownFnForTest(ws, func() {})
	events := ws.Events(t.Context())
	older := attachedClient(t, b, ws)
	newer := attachedClient(t, b, ws)

	_, err := b.SetLSPHost(ws.ID, older, testLSPHost("older"))
	require.NoError(t, err)
	require.Equal(t, older, nextLSPHostEvent(t, events).Payload.ClientID)

	st, err := b.SetLSPHost(ws.ID, newer, testLSPHost("newer"))
	require.NoError(t, err)
	require.Equal(t, newer, st.ClientID)
	ev := nextLSPHostEvent(t, events)
	require.Equal(t, pubsub.UpdatedEvent, ev.Type)
	require.Equal(t, newer, ev.Payload.ClientID)
	require.Equal(t, "newer", ev.Payload.LSPHost.Name)
	requireLSPHost(t, b, ws, newer, testLSPHost("newer"))

	// The older client saying the same thing again does not make its
	// registration the newest, and its answer is its own registration.
	st, err = b.SetLSPHost(ws.ID, older, testLSPHost("older"))
	require.NoError(t, err)
	require.Equal(t, older, st.ClientID)
	require.Equal(t, "older", st.LSPHost.Name)
	requireNoLSPHostEvent(t, events)
	requireLSPHost(t, b, ws, newer, testLSPHost("newer"))

	// The newest clears its registration: the older one is in effect
	// again, without registering again.
	_, err = b.SetLSPHost(ws.ID, newer, nil)
	require.NoError(t, err)
	ev = nextLSPHostEvent(t, events)
	require.Equal(t, pubsub.UpdatedEvent, ev.Type)
	require.Equal(t, older, ev.Payload.ClientID)
	require.Equal(t, "older", ev.Payload.LSPHost.Name)
	requireLSPHost(t, b, ws, older, testLSPHost("older"))

	// The newest retires, as a host that exits does.
	_, err = b.SetLSPHost(ws.ID, newer, testLSPHost("newer"))
	require.NoError(t, err)
	require.Equal(t, newer, nextLSPHostEvent(t, events).Payload.ClientID)
	require.NoError(t, b.RetireClient(newer))
	ev = nextLSPHostEvent(t, events)
	require.Equal(t, pubsub.UpdatedEvent, ev.Type)
	require.Equal(t, older, ev.Payload.ClientID)
	requireLSPHost(t, b, ws, older, testLSPHost("older"))

	// A client that is not the host leaving changes nothing.
	bystander := attachedClient(t, b, ws)
	require.NoError(t, b.RetireClient(bystander))
	requireNoLSPHostEvent(t, events)

	// The last host retires: the workspace has none. A third client
	// keeps the workspace itself alive.
	keeper := attachedClient(t, b, ws)
	require.NoError(t, b.RetireClient(older))
	ev = nextLSPHostEvent(t, events)
	require.Equal(t, pubsub.DeletedEvent, ev.Type)
	require.Equal(t, proto.LSPHostState{WorkspaceID: ws.ID}, ev.Payload)
	requireLSPHost(t, b, ws, "", nil)
	_ = keeper
}

// TestLSPHost_GoesWithTheStream covers a host that crashes: its stream
// drops without a goodbye, and its registration lasts as long as its
// claim, which is the detach grace. A host that reconnects in time keeps
// it.
func TestLSPHost_GoesWithTheStream(t *testing.T) {
	t.Parallel()
	b, ws, _ := newPromptsTestBackend(t)
	SetWorkspaceShutdownFnForTest(ws, func() {})
	b.mu.Lock()
	b.detachGrace = 300 * time.Millisecond
	b.mu.Unlock()
	events := ws.Events(t.Context())
	attachedClient(t, b, ws) // Keeps the workspace alive.
	host := attachedClient(t, b, ws)

	_, err := b.SetLSPHost(ws.ID, host, testLSPHost("joy"))
	require.NoError(t, err)
	nextLSPHostEvent(t, events)

	// A drop and a reconnect within the grace: nothing changes.
	b.DetachClient(ws.ID, host)
	requireLSPHost(t, b, ws, host, testLSPHost("joy"))
	require.NoError(t, b.AttachClient(ws.ID, host))
	time.Sleep(400 * time.Millisecond)
	requireLSPHost(t, b, ws, host, testLSPHost("joy"))

	// A drop for good: the registration expires with the claim.
	b.DetachClient(ws.ID, host)
	ev := nextLSPHostEvent(t, events)
	require.Equal(t, pubsub.DeletedEvent, ev.Type)
	requireLSPHost(t, b, ws, "", nil)

	// Without a claim the client has to attach before it registers again.
	_, err = b.SetLSPHost(ws.ID, host, testLSPHost("joy"))
	require.ErrorIs(t, err, ErrClientNoClaim)
}

// TestLSPHost_ReachesTheLSPManager verifies that the workspace's LSP
// manager follows the effective host.
func TestLSPHost_ReachesTheLSPManager(t *testing.T) {
	t.Parallel()
	b, ws, a := newPromptsTestBackend(t)
	SetWorkspaceShutdownFnForTest(ws, func() {})
	a.LSPManager = lsp.NewManager(config.NewTestStoreIn(ws.Path, &config.Config{Options: &config.Options{}}))
	older := attachedClient(t, b, ws)
	newer := attachedClient(t, b, ws)

	require.Nil(t, a.LSPManager.Host())

	_, err := b.SetLSPHost(ws.ID, older, testLSPHost("older"))
	require.NoError(t, err)
	require.Equal(t, &lsp.Host{
		Name:      "older",
		Version:   "1.2.3",
		PID:       4242,
		Socket:    "/run/older/lsp.sock",
		Workspace: ws.ID,
		Servers: []lsp.HostServer{
			{Name: "gopls", Command: "gopls"},
			{Name: "rust-analyzer", Command: "rust-analyzer"},
		},
	}, a.LSPManager.Host())

	_, err = b.SetLSPHost(ws.ID, newer, testLSPHost("newer"))
	require.NoError(t, err)
	require.Equal(t, "/run/newer/lsp.sock", a.LSPManager.Host().Socket)

	require.NoError(t, b.RetireClient(newer))
	require.Equal(t, "/run/older/lsp.sock", a.LSPManager.Host().Socket)

	require.NoError(t, b.RetireClient(older))
	require.Nil(t, a.LSPManager.Host())
}

// Without CRUSH_LSP_HOST in its environment, a workspace keeps what hosts
// register but uses none: its own environment decides, over the server's.
func TestLSPHost_NeedsTheEnvironmentVariable(t *testing.T) {
	t.Parallel()
	b, ws, _ := newPromptsTestBackend(t)
	ws.Env = []string{"PATH=/bin", LSPHostEnv + "=0"}
	events := ws.Events(t.Context())
	client := attachedClient(t, b, ws)

	st, err := b.SetLSPHost(ws.ID, client, testLSPHost("joy"))
	require.NoError(t, err)
	require.True(t, testLSPHost("joy").Equal(st.LSPHost), "the registration is kept")
	requireNoLSPHostEvent(t, events)
	requireLSPHost(t, b, ws, "", nil)

	require.False(t, usesLSPHost(&Workspace{Env: []string{LSPHostEnv + "=false"}}))
	require.True(t, usesLSPHost(&Workspace{Env: []string{LSPHostEnv + "=0", LSPHostEnv + "=1"}}))
	require.True(t, usesLSPHost(&Workspace{}), "the server's own environment, set for these tests")
}
