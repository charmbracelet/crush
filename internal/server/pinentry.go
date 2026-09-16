package server

import (
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/crush/internal/proto"
)

// handlePostWorkspacePinentryAnswer resolves the pending integrated
// pinentry prompt with the user's secret.
//
//	@Summary		Answer pinentry prompt
//	@Tags			pinentry
//	@Param			id		path	string					true	"Workspace ID"
//	@Param			request	body	proto.PinentryAnswer	true	"Pinentry answer"
//	@Success		200	{object}	proto.PinentryResponse
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/pinentry/answer [post]
func (c *controllerV1) handlePostWorkspacePinentryAnswer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.PinentryAnswer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	resolved, err := c.backend.AnswerPinentry(id, req)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.PinentryResponse{Resolved: resolved})
}

// handlePostWorkspacePinentryCancel dismisses the pending integrated
// pinentry prompt for a workspace.
//
//	@Summary		Cancel pinentry prompt
//	@Tags			pinentry
//	@Param			id		path	string					true	"Workspace ID"
//	@Param			request	body	proto.PinentryCancel	true	"Pinentry cancel"
//	@Success		200	{object}	proto.PinentryResponse
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/pinentry/cancel [post]
func (c *controllerV1) handlePostWorkspacePinentryCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.PinentryCancel
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	cancelled, err := c.backend.CancelPinentry(id, req)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.PinentryResponse{Resolved: cancelled})
}
