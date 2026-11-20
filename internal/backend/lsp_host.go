package backend

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsphost"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// GetLSPHost returns the workspace's LSP host: the newest registration
// among the clients that hold a claim on the workspace, with the client it
// belongs to. A nil LSPHost means no client registered one.
func (b *Backend) GetLSPHost(workspaceID string) (proto.LSPHostState, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.LSPHostState{}, err
	}
	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	clientID, h := effectiveLSPHost(ws)
	return proto.LSPHostState{WorkspaceID: ws.ID, ClientID: clientID, LSPHost: h}, nil
}

// SetLSPHost registers the application behind clientID as an LSP host of
// the workspace: an application, such as an editor running Crush in a
// pane, that shares its language servers. A nil h clears the client's
// registration. It returns the client's registration after the call.
//
// The registration belongs to the client's claim on the workspace, so the
// client must hold one ([ErrClientNoClaim] otherwise), and the registration
// goes away with the claim: when the client retires, or when its event
// stream stays closed for longer than the detach grace. A host that exits
// or crashes is therefore forgotten without having to say so.
//
// The workspace's host is the newest registration. Whenever that changes,
// an lsp_host event is published (updated, or deleted when no registration
// is left) and the workspace's LSP manager is told. Registering the host a
// client already has is a no-op. Invalid hosts fail with
// [ErrInvalidLSPHost].
func (b *Backend) SetLSPHost(workspaceID, clientID string, h *proto.LSPHost) (proto.LSPHostState, error) {
	if _, err := validateClientID(clientID); err != nil {
		return proto.LSPHostState{}, err
	}
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.LSPHostState{}, err
	}
	if h != nil {
		if err := validateLSPHost(h); err != nil {
			return proto.LSPHostState{}, fmt.Errorf("%w: %w", ErrInvalidLSPHost, err)
		}
		h = h.Clone()
	}

	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	cs, ok := ws.clients[clientID]
	if !ok {
		return proto.LSPHostState{}, ErrClientNoClaim
	}
	if !cs.lspHost.Equal(h) {
		cs.lspHost, cs.lspHostSeq = h, 0
		if h != nil {
			ws.lspHostSeq++
			cs.lspHostSeq = ws.lspHostSeq
		}
		syncLSPHost(ws)
	}
	return proto.LSPHostState{WorkspaceID: ws.ID, ClientID: clientID, LSPHost: cs.lspHost}, nil
}

// validateLSPHost checks what Crush relies on in a host registration.
func validateLSPHost(h *proto.LSPHost) error {
	if h.Protocol != proto.LSPHostProtocol {
		return fmt.Errorf("protocol %d is not supported, this is protocol %d", h.Protocol, proto.LSPHostProtocol)
	}
	if _, err := lsphost.SocketPath(h.Address); err != nil {
		return err
	}
	if len(h.Servers) > proto.MaxLSPHostServers {
		return fmt.Errorf("%d servers, at most %d are allowed", len(h.Servers), proto.MaxLSPHostServers)
	}
	for i, s := range h.Servers {
		if s.Name == "" || s.Command == "" {
			return fmt.Errorf("servers[%d] needs a name and a command", i)
		}
	}
	return nil
}

// effectiveLSPHost returns the workspace's LSP host and the client it
// belongs to: the newest registration among the current claims, so that
// when the newest host leaves, an older one that is still there takes
// over. It is called with clientsMu held.
func effectiveLSPHost(ws *Workspace) (string, *proto.LSPHost) {
	if !usesLSPHost(ws) {
		return "", nil
	}
	var (
		clientID string
		newest   *clientState
	)
	for id, cs := range ws.clients {
		if cs.lspHost != nil && (newest == nil || cs.lspHostSeq > newest.lspHostSeq) {
			clientID, newest = id, cs
		}
	}
	if newest == nil {
		return "", nil
	}
	return clientID, newest.lspHost
}

// LSPHostEnv is the environment variable that lets a workspace use the
// language servers a host application shares: CRUSH_LSP_HOST=1 in the
// environment of the client that created the workspace, which is how a
// host starts Crush when it shares its servers, or in the server's own.
// Without it, registrations are kept but no host is in effect, and Crush
// runs its own servers.
const LSPHostEnv = "CRUSH_LSP_HOST"

// usesLSPHost reports whether the workspace uses a host's language
// servers: see [LSPHostEnv]. The workspace's environment wins over the
// server's.
func usesLSPHost(ws *Workspace) bool {
	value := os.Getenv(LSPHostEnv)
	for _, kv := range ws.Env {
		if v, ok := strings.CutPrefix(kv, LSPHostEnv+"="); ok {
			value = v
		}
	}
	on, _ := strconv.ParseBool(value)
	return on
}

// syncLSPHost makes the workspace's LSP host known after a registration
// was added, replaced or removed: if the effective host changed, it
// publishes an lsp_host event and tells the LSP manager. It is called with
// clientsMu held, so changes are announced in the order they happen;
// neither publishing nor the manager blocks.
func syncLSPHost(ws *Workspace) {
	clientID, h := effectiveLSPHost(ws)
	// Stored hosts are never mutated, only replaced, so the same pointer
	// is the same registration.
	if h == ws.lspHost {
		return
	}
	ws.lspHost = h
	if ws.App == nil { // Synthetic test workspaces have no App.
		return
	}
	typ := pubsub.UpdatedEvent
	if h == nil {
		typ = pubsub.DeletedEvent
	}
	ws.SendEvent(pubsub.Event[proto.LSPHostState]{
		Type:    typ,
		Payload: proto.LSPHostState{WorkspaceID: ws.ID, ClientID: clientID, LSPHost: h},
	})
	ws.LSPManager.SetHost(lspHostFor(ws.ID, h))
}

// lspHostFor converts a registration to what the LSP manager works with.
func lspHostFor(workspaceID string, h *proto.LSPHost) *lsp.Host {
	if h == nil {
		return nil
	}
	socket, err := lsphost.SocketPath(h.Address)
	if err != nil {
		return nil // Unreachable: registrations are validated.
	}
	host := &lsp.Host{
		Name:      h.Name,
		Version:   h.Version,
		PID:       h.PID,
		Socket:    socket,
		Workspace: workspaceID,
		Servers:   make([]lsp.HostServer, len(h.Servers)),
	}
	for i, s := range h.Servers {
		host.Servers[i] = lsp.HostServer{Name: s.Name, Command: s.Command}
	}
	return host
}
