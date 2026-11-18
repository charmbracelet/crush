package server

import (
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/crush/internal/proto"
)

// handlePostWorkspaceSSHAnswer resolves the pending integrated SSH
// prompt with the user's secret.
//
//	@Summary		Answer SSH prompt
//	@Tags			ssh
//	@Param			id		path	string				true	"Workspace ID"
//	@Param			request	body	proto.SSHAnswer	true	"SSH answer"
//	@Success		200	{object}	proto.SSHResponse
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/ssh/answer [post]
func (c *controllerV1) handlePostWorkspaceSSHAnswer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.SSHAnswer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	resolved, err := c.backend.AnswerSSH(id, req)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.SSHResponse{Resolved: resolved})
}

// handlePostWorkspaceSSHCancel dismisses the pending integrated SSH
// prompt for a workspace.
//
//	@Summary		Cancel SSH prompt
//	@Tags			ssh
//	@Param			id		path	string				true	"Workspace ID"
//	@Param			request	body	proto.SSHCancel	true	"SSH cancel"
//	@Success		200	{object}	proto.SSHResponse
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/ssh/cancel [post]
func (c *controllerV1) handlePostWorkspaceSSHCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.SSHCancel
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	cancelled, err := c.backend.CancelSSH(id, req)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.SSHResponse{Resolved: cancelled})
}
