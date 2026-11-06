package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// maxHostThemeBody bounds the body of a host theme request. A complete
// theme (every palette and syntax key, generous names) is a few KiB.
const maxHostThemeBody = 64 << 10

// handleGetWorkspaceTheme returns the workspace's host theme.
func (c *controllerV1) handleGetWorkspaceTheme(w http.ResponseWriter, r *http.Request) {
	st, err := c.backend.GetHostTheme(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, st)
}

// handlePostWorkspaceTheme sets or, with a null theme, clears the
// workspace's host theme and responds with the resulting state. A body
// without the "theme" key is rejected rather than read as a clear, so a
// theme sent without its wrapper cannot wipe the current one.
func (c *controllerV1) handlePostWorkspaceTheme(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, maxHostThemeBody)
	// The theme stays raw at first so a missing key can be told apart
	// from an explicit null. The wire shape is [proto.HostThemeRequest].
	var req struct {
		Theme json.RawMessage `json:"theme"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			jsonError(w, http.StatusRequestEntityTooLarge, "theme request is too large")
			return
		}
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}
	if len(req.Theme) == 0 {
		jsonError(w, http.StatusBadRequest, `missing theme; send {"theme": null} to clear it`)
		return
	}
	var theme *proto.HostTheme
	if err := json.Unmarshal(req.Theme, &theme); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}
	st, err := c.backend.SetHostTheme(id, theme)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, st)
}

// replayHostTheme writes the workspace's current host theme, if one is
// set, to a client that just attached its event stream. A client that
// starts or reconnects after the host set its theme would otherwise keep
// its own colors until the host changes theme again. It runs after the
// stream subscribed, so an update racing the attach is delivered again by
// the event loop rather than lost; clients drop the duplicate.
func (c *controllerV1) replayHostTheme(w http.ResponseWriter, r *http.Request, flusher *http.ResponseController, workspaceID string) {
	st, err := c.backend.GetHostTheme(workspaceID)
	if err != nil || st.Theme == nil {
		return
	}
	c.writeEvent(w, r, flusher, pubsub.Event[proto.HostThemeState]{
		Type:    pubsub.UpdatedEvent,
		Payload: st,
	})
}

// replayClientSessions writes the session each client of the workspace
// views to a client that just attached its event stream, so that it knows
// the sessions of the clients that were already there. It runs after the
// stream subscribed, like replayHostTheme.
func (c *controllerV1) replayClientSessions(w http.ResponseWriter, r *http.Request, flusher *http.ResponseController, workspaceID string) {
	sessions, err := c.backend.ClientSessions(workspaceID)
	if err != nil {
		return
	}
	for _, s := range sessions {
		c.writeEvent(w, r, flusher, pubsub.Event[proto.ClientSession]{
			Type:    pubsub.UpdatedEvent,
			Payload: s,
		})
	}
}
