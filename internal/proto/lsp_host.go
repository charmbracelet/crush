package proto

import "slices"

// LSPHostProtocol is the version of the LSP host protocol Crush speaks.
const LSPHostProtocol = 1

// MaxLSPHostServers is the most servers a host may offer.
const MaxLSPHostServers = 64

// LSPHost is an application, such as an editor that runs Crush in one of
// its panes, that shares its language servers with Crush. Crush connects
// to a server the host offers instead of starting its own, so both work on
// one set of servers.
//
// A registration is never persisted and belongs to the client that sent
// it: it goes away with that client's claim on the workspace.
type LSPHost struct {
	// Protocol is the version of the LSP host protocol the host speaks.
	Protocol int `json:"protocol"`
	// Name names the host application. It is informational only.
	Name string `json:"name,omitempty"`
	// Version is the host application's version. It is informational only.
	Version string `json:"version,omitempty"`
	// PID is the process that announced the host. It is informational
	// only: the address may be served by another process of the host.
	PID int `json:"pid,omitempty"`
	// Address is where the host listens: "unix://" followed by the
	// absolute path of a unix socket.
	Address string `json:"address"`
	// Servers are the language servers the host offers.
	Servers []LSPHostServer `json:"servers"`
}

// LSPHostServer is a language server an [LSPHost] offers.
type LSPHostServer struct {
	// Name is the host's name for the server. Crush sends it back when it
	// connects.
	Name string `json:"name"`
	// Command is the server's command. Crush uses the host's server in
	// place of its own server with the same command name.
	Command string `json:"command"`
}

// Equal reports whether h and o describe the same host. Two nil hosts are
// equal.
func (h *LSPHost) Equal(o *LSPHost) bool {
	if h == nil || o == nil {
		return h == o
	}
	return h.Protocol == o.Protocol &&
		h.Name == o.Name &&
		h.Version == o.Version &&
		h.PID == o.PID &&
		h.Address == o.Address &&
		slices.Equal(h.Servers, o.Servers)
}

// Clone returns a deep copy of h, or nil when h is nil.
func (h *LSPHost) Clone() *LSPHost {
	if h == nil {
		return nil
	}
	c := *h
	c.Servers = slices.Clone(h.Servers)
	return &c
}

// LSPHostRequest is the body of POST /v1/workspaces/{id}/lsp-host. A nil
// LSPHost clears the calling client's registration.
type LSPHostRequest struct {
	LSPHost *LSPHost `json:"lsp_host"`
}

// LSPHostState is an LSP host registration of a workspace. It is the
// response of the lsp-host endpoints and the payload of lsp_host events
// (an updated event when the workspace's host changes, a deleted one when
// it is left without any). A nil LSPHost means none; ClientID is the
// client the registration belongs to.
type LSPHostState struct {
	WorkspaceID string   `json:"workspace_id"`
	ClientID    string   `json:"client_id,omitempty"`
	LSPHost     *LSPHost `json:"lsp_host"`
}
