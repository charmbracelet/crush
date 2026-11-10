package server

import (
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/crush/internal/proto"
)

// handlePostWorkspaceTerminalResult resolves a terminal call the agent
// forwarded to the calling client with the result of running it there.
//
//	@Summary		Answer terminal call
//	@Tags			terminal
//	@Param			id			path	string					true	"Workspace ID"
//	@Param			client_id	query	string					true	"Client ID"
//	@Param			request		body	proto.TerminalResult	true	"Terminal result"
//	@Success		200	{object}	proto.TerminalResponse
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/terminal/result [post]
func (c *controllerV1) handlePostWorkspaceTerminalResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	clientID, ok := c.requireClientID(w, r)
	if !ok {
		return
	}

	var req proto.TerminalResult
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	resolved, err := c.backend.RespondTerminal(id, clientID, req)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.TerminalResponse{Resolved: resolved})
}

// handlePostWorkspaceAgentSessionShellRecord stores a shell command the
// user ran on the client's machine as a message in the session.
//
//	@Summary		Record shell command
//	@Tags			agent
//	@Param			id		path	string						true	"Workspace ID"
//	@Param			sid		path	string						true	"Session ID"
//	@Param			request	body	proto.ShellCommandRecord	true	"Shell command"
//	@Success		200
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/sessions/{sid}/shell/record [post]
func (c *controllerV1) handlePostWorkspaceAgentSessionShellRecord(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")

	var req proto.ShellCommandRecord
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.RecordShellCommand(r.Context(), id, sid, req); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
