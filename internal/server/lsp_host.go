package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// maxLSPHostBody bounds the body of an LSP host request. A registration
// with the most servers allowed is a few KiB.
const maxLSPHostBody = 64 << 10

// handleGetWorkspaceLSPHost returns the workspace's LSP host.
func (c *controllerV1) handleGetWorkspaceLSPHost(w http.ResponseWriter, r *http.Request) {
	st, err := c.backend.GetLSPHost(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, st)
}

// handlePostWorkspaceLSPHost registers the calling client's application
// as an LSP host of the workspace or, with a null lsp_host, clears its
// registration, and responds with the client's registration. A body
// without the "lsp_host" key is rejected rather than read as a clear, as
// the theme endpoint does.
func (c *controllerV1) handlePostWorkspaceLSPHost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	clientID, ok := c.requireClientID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLSPHostBody)
	// The host stays raw at first so a missing key can be told apart from
	// an explicit null. The wire shape is [proto.LSPHostRequest].
	var req struct {
		LSPHost json.RawMessage `json:"lsp_host"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			jsonError(w, http.StatusRequestEntityTooLarge, "LSP host request is too large")
			return
		}
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}
	if len(req.LSPHost) == 0 {
		jsonError(w, http.StatusBadRequest, `missing lsp_host; send {"lsp_host": null} to clear it`)
		return
	}
	var host *proto.LSPHost
	if err := json.Unmarshal(req.LSPHost, &host); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}
	st, err := c.backend.SetLSPHost(id, clientID, host)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, st)
}

// replayLSPHost writes the workspace's LSP host, if there is one, to a
// client that just attached its event stream, so that a client starting
// after the host announced itself knows about it. It runs after the stream
// subscribed, like replayHostTheme, so a change racing the attach is
// delivered again by the event loop rather than lost.
func (c *controllerV1) replayLSPHost(w http.ResponseWriter, r *http.Request, flusher *http.ResponseController, workspaceID string) {
	st, err := c.backend.GetLSPHost(workspaceID)
	if err != nil || st.LSPHost == nil {
		return
	}
	c.writeEvent(w, r, flusher, pubsub.Event[proto.LSPHostState]{
		Type:    pubsub.UpdatedEvent,
		Payload: st,
	})
}
