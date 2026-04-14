package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/session"
)

type controllerV1 struct {
	backend *backend.Backend
	server  *Server
}

// handleGetHealth checks server health.
//
//	@Summary		Health check
//	@Tags			system
//	@Success		200
//	@Router			/health [get]
func (c *controllerV1) handleGetHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleGetVersion returns server version information.
//
//	@Summary		Get server version
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	proto.VersionInfo
//	@Router			/version [get]
func (c *controllerV1) handleGetVersion(w http.ResponseWriter, _ *http.Request) {
	jsonEncode(w, c.backend.VersionInfo())
}

// handlePostControl sends a control command to the server.
//
//	@Summary		Send server control command
//	@Tags			system
//	@Accept			json
//	@Param			request	body	proto.ServerControl	true	"Control command (e.g. shutdown)"
//	@Success		200
//	@Failure		400	{object}	proto.Error
//	@Router			/control [post]
func (c *controllerV1) handlePostControl(w http.ResponseWriter, r *http.Request) {
	var req proto.ServerControl
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	switch req.Command {
	case "shutdown":
		c.backend.Shutdown()
	default:
		c.server.logError(r, "Unknown command", "command", req.Command)
		jsonError(w, http.StatusBadRequest, "unknown command")
		return
	}
}

// handleGetConfig returns global server configuration.
//
//	@Summary		Get server config
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	object
//	@Router			/config [get]
func (c *controllerV1) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	jsonEncode(w, c.backend.Config())
}

// handleGetWorkspaces lists all workspaces.
//
//	@Summary		List workspaces
//	@Tags			workspaces
//	@Produce		json
//	@Success		200	{array}		proto.Workspace
//	@Router			/workspaces [get]
func (c *controllerV1) handleGetWorkspaces(w http.ResponseWriter, _ *http.Request) {
	jsonEncode(w, c.backend.ListWorkspaces())
}

// handleGetWorkspace returns a single workspace by ID.
//
//	@Summary		Get workspace
//	@Tags			workspaces
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{object}	proto.Workspace
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id} [get]
func (c *controllerV1) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ws, err := c.backend.GetWorkspaceProto(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, ws)
}

// handlePostWorkspaces creates a new workspace.
//
//	@Summary		Create workspace
//	@Tags			workspaces
//	@Accept			json
//	@Produce		json
//	@Param			request	body		proto.Workspace	true	"Workspace creation params"
//	@Success		200		{object}	proto.Workspace
//	@Failure		400		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces [post]
func (c *controllerV1) handlePostWorkspaces(w http.ResponseWriter, r *http.Request) {
	var args proto.Workspace
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	_, result, err := c.backend.CreateWorkspace(args)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}

// handleDeleteWorkspaces deletes a workspace.
//
//	@Summary		Delete workspace
//	@Tags			workspaces
//	@Param			id	path	string	true	"Workspace ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Router			/workspaces/{id} [delete]
func (c *controllerV1) handleDeleteWorkspaces(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c.backend.DeleteWorkspace(id)
}

// handleGetWorkspaceConfig returns workspace configuration.
//
//	@Summary		Get workspace config
//	@Tags			workspaces
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{object}	object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/config [get]
func (c *controllerV1) handleGetWorkspaceConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg, err := c.backend.GetWorkspaceConfig(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, cfg)
}

// handleGetWorkspaceProviders lists available providers for a workspace.
//
//	@Summary		Get workspace providers
//	@Tags			workspaces
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{object}	object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/providers [get]
func (c *controllerV1) handleGetWorkspaceProviders(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	providers, err := c.backend.GetWorkspaceProviders(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, providers)
}

// handleGetWorkspaceEvents streams workspace events as Server-Sent Events.
//
//	@Summary		Stream workspace events (SSE)
//	@Tags			workspaces
//	@Produce		text/event-stream
//	@Param			id	path	string	true	"Workspace ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/events [get]
func (c *controllerV1) handleGetWorkspaceEvents(w http.ResponseWriter, r *http.Request) {
	flusher := http.NewResponseController(w)
	id := r.PathValue("id")
	events, err := c.backend.SubscribeEvents(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case <-r.Context().Done():
			c.server.logDebug(r, "Stopping event stream")
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			c.server.logDebug(r, "Sending event", "event", fmt.Sprintf("%T %+v", ev, ev))
			wrapped := wrapEvent(ev)
			if wrapped == nil {
				continue
			}
			data, err := json.Marshal(wrapped)
			if err != nil {
				c.server.logError(r, "Failed to marshal event", "error", err)
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// handleGetWorkspaceLSPs lists LSP clients for a workspace.
//
//	@Summary		List LSP clients
//	@Tags			lsp
//	@Produce		json
//	@Param			id	path		string							true	"Workspace ID"
//	@Success		200	{object}	map[string]proto.LSPClientInfo
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/lsps [get]
func (c *controllerV1) handleGetWorkspaceLSPs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	states, err := c.backend.GetLSPStates(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	result := make(map[string]proto.LSPClientInfo, len(states))
	for k, v := range states {
		result[k] = proto.LSPClientInfo{
			Name:            v.Name,
			State:           v.State,
			Error:           v.Error,
			DiagnosticCount: v.DiagnosticCount,
			ConnectedAt:     v.ConnectedAt,
		}
	}
	jsonEncode(w, result)
}

// handleGetWorkspaceLSPDiagnostics returns diagnostics for an LSP client.
//
//	@Summary		Get LSP diagnostics
//	@Tags			lsp
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Param			lsp	path		string	true	"LSP client name"
//	@Success		200	{object}	object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/lsps/{lsp}/diagnostics [get]
func (c *controllerV1) handleGetWorkspaceLSPDiagnostics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lspName := r.PathValue("lsp")
	diagnostics, err := c.backend.GetLSPDiagnostics(id, lspName)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, diagnostics)
}

// handleGetWorkspaceSessions lists sessions for a workspace.
//
//	@Summary		List sessions
//	@Tags			sessions
//	@Produce		json
//	@Param			id	path		string			true	"Workspace ID"
//	@Success		200	{array}		proto.Session
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions [get]
func (c *controllerV1) handleGetWorkspaceSessions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sessions, err := c.backend.ListSessions(r.Context(), id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	result := make([]proto.Session, len(sessions))
	for i, s := range sessions {
		result[i] = sessionToProto(s)
	}
	jsonEncode(w, result)
}

// handlePostWorkspaceSessions creates a new session in a workspace.
//
//	@Summary		Create session
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"Workspace ID"
//	@Param			request	body		proto.Session	true	"Session creation params (title)"
//	@Success		200		{object}	proto.Session
//	@Failure		400		{object}	proto.Error
//	@Failure		404		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces/{id}/sessions [post]
func (c *controllerV1) handlePostWorkspaceSessions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var args session.Session
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	sess, err := c.backend.CreateSession(r.Context(), id, args.Title)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, sessionToProto(sess))
}

// handleGetWorkspaceSession returns a single session.
//
//	@Summary		Get session
//	@Tags			sessions
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Param			sid	path		string	true	"Session ID"
//	@Success		200	{object}	proto.Session
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid} [get]
func (c *controllerV1) handleGetWorkspaceSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	sess, err := c.backend.GetSession(r.Context(), id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, sessionToProto(sess))
}

// handleGetWorkspaceSessionHistory returns the history for a session.
//
//	@Summary		Get session history
//	@Tags			sessions
//	@Produce		json
//	@Param			id	path		string		true	"Workspace ID"
//	@Param			sid	path		string		true	"Session ID"
//	@Success		200	{array}		proto.File
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/history [get]
func (c *controllerV1) handleGetWorkspaceSessionHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	history, err := c.backend.ListSessionHistory(r.Context(), id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, history)
}

// handleGetWorkspaceSessionMessages returns all messages for a session.
//
//	@Summary		Get session messages
//	@Tags			sessions
//	@Produce		json
//	@Param			id	path		string			true	"Workspace ID"
//	@Param			sid	path		string			true	"Session ID"
//	@Success		200	{array}		proto.Message
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/messages [get]
func (c *controllerV1) handleGetWorkspaceSessionMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	messages, err := c.backend.ListSessionMessages(r.Context(), id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, messagesToProto(messages))
}

// handlePutWorkspaceSession updates a session.
//
//	@Summary		Update session
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"Workspace ID"
//	@Param			sid		path		string			true	"Session ID"
//	@Param			request	body		proto.Session	true	"Updated session"
//	@Success		200		{object}	proto.Session
//	@Failure		400		{object}	proto.Error
//	@Failure		404		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid} [put]
func (c *controllerV1) handlePutWorkspaceSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var sess session.Session
	if err := json.NewDecoder(r.Body).Decode(&sess); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	saved, err := c.backend.SaveSession(r.Context(), id, sess)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, sessionToProto(saved))
}

// handleDeleteWorkspaceSession deletes a session.
//
//	@Summary		Delete session
//	@Tags			sessions
//	@Param			id	path	string	true	"Workspace ID"
//	@Param			sid	path	string	true	"Session ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid} [delete]
func (c *controllerV1) handleDeleteWorkspaceSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	if err := c.backend.DeleteSession(r.Context(), id, sid); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceSessionModels updates the models for a session.
//
//	@Summary		Update session models
//	@Tags			sessions
//	@Accept			json
//	@Param			id		path		string										true	"Workspace ID"
//	@Param			sid		path		string										true	"Session ID"
//	@Param			body	body		map[config.SelectedModelType]config.SelectedModel	true	"Models"
//	@Success		204
//	@Failure		400	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/models [post]
func (c *controllerV1) handlePostWorkspaceSessionModels(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("id")
	sessionID := r.PathValue("sid")

	var req struct {
		Models map[config.SelectedModelType]config.SelectedModel `json:"models"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.UpdateSessionModels(r.Context(), workspaceID, sessionID, req.Models); err != nil {
		c.handleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetWorkspaceSessionUserMessages returns user messages for a session.
//
//	@Summary		Get user messages for session
//	@Tags			sessions
//	@Produce		json
//	@Param			id	path		string			true	"Workspace ID"
//	@Param			sid	path		string			true	"Session ID"
//	@Success		200	{array}		proto.Message
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/messages/user [get]
func (c *controllerV1) handleGetWorkspaceSessionUserMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	messages, err := c.backend.ListUserMessages(r.Context(), id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, messagesToProto(messages))
}

// handleGetWorkspaceAllUserMessages returns all user messages across sessions.
//
//	@Summary		Get all user messages for workspace
//	@Tags			workspaces
//	@Produce		json
//	@Param			id	path		string			true	"Workspace ID"
//	@Success		200	{array}		proto.Message
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/messages/user [get]
func (c *controllerV1) handleGetWorkspaceAllUserMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	messages, err := c.backend.ListAllUserMessages(r.Context(), id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, messagesToProto(messages))
}

// handleGetWorkspaceSessionFileTrackerFiles lists files read in a session.
//
//	@Summary		List tracked files for session
//	@Tags			filetracker
//	@Produce		json
//	@Param			id	path		string		true	"Workspace ID"
//	@Param			sid	path		string		true	"Session ID"
//	@Success		200	{array}		string
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/filetracker/files [get]
func (c *controllerV1) handleGetWorkspaceSessionFileTrackerFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	files, err := c.backend.FileTrackerListReadFiles(r.Context(), id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, files)
}

// handlePostWorkspaceFileTrackerRead records a file read event.
//
//	@Summary		Record file read
//	@Tags			filetracker
//	@Accept			json
//	@Param			id		path	string							true	"Workspace ID"
//	@Param			request	body	proto.FileTrackerReadRequest	true	"File tracker read request"
//	@Success		200
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/filetracker/read [post]
func (c *controllerV1) handlePostWorkspaceFileTrackerRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.FileTrackerReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.FileTrackerRecordRead(r.Context(), id, req.SessionID, req.Path); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetWorkspaceFileTrackerLastRead returns the last read time for a file.
//
//	@Summary		Get last read time for file
//	@Tags			filetracker
//	@Produce		json
//	@Param			id			path		string	true	"Workspace ID"
//	@Param			session_id	query		string	false	"Session ID"
//	@Param			path		query		string	true	"File path"
//	@Success		200			{object}	object
//	@Failure		404			{object}	proto.Error
//	@Failure		500			{object}	proto.Error
//	@Router			/workspaces/{id}/filetracker/lastread [get]
func (c *controllerV1) handleGetWorkspaceFileTrackerLastRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.URL.Query().Get("session_id")
	path := r.URL.Query().Get("path")

	t, err := c.backend.FileTrackerLastReadTime(r.Context(), id, sid, path)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, t)
}

// handlePostWorkspaceLSPStart starts an LSP server for a path.
//
//	@Summary		Start LSP server
//	@Tags			lsp
//	@Accept			json
//	@Param			id		path	string					true	"Workspace ID"
//	@Param			request	body	proto.LSPStartRequest	true	"LSP start request"
//	@Success		200
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/lsps/start [post]
func (c *controllerV1) handlePostWorkspaceLSPStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.LSPStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.LSPStart(r.Context(), id, req.Path); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceLSPStopAll stops all LSP servers.
//
//	@Summary		Stop all LSP servers
//	@Tags			lsp
//	@Param			id	path	string	true	"Workspace ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/lsps/stop [post]
func (c *controllerV1) handlePostWorkspaceLSPStopAll(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := c.backend.LSPStopAll(r.Context(), id); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetWorkspaceAgent returns agent info for a workspace.
//
//	@Summary		Get agent info
//	@Tags			agent
//	@Produce		json
//	@Param			id	path		string			true	"Workspace ID"
//	@Success		200	{object}	proto.AgentInfo
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent [get]
func (c *controllerV1) handleGetWorkspaceAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, err := c.backend.GetAgentInfo(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, info)
}

// handlePostWorkspaceAgent sends a message to the agent.
//
//	@Summary		Send message to agent
//	@Tags			agent
//	@Accept			json
//	@Param			id		path	string				true	"Workspace ID"
//	@Param			request	body	proto.AgentMessage	true	"Agent message"
//	@Success		200
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent [post]
func (c *controllerV1) handlePostWorkspaceAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var msg proto.AgentMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.SendMessage(r.Context(), id, msg); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceAgentInit initializes the agent for a workspace.
//
//	@Summary		Initialize agent
//	@Tags			agent
//	@Param			id	path	string	true	"Workspace ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/init [post]
func (c *controllerV1) handlePostWorkspaceAgentInit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := c.backend.InitAgent(r.Context(), id); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceAgentUpdate updates the agent for a workspace.
//
//	@Summary		Update agent
//	@Tags			agent
//	@Param			id	path	string	true	"Workspace ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/update [post]
func (c *controllerV1) handlePostWorkspaceAgentUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := c.backend.UpdateAgent(r.Context(), id); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetWorkspaceAgentSession returns a specific agent session.
//
//	@Summary		Get agent session
//	@Tags			agent
//	@Produce		json
//	@Param			id	path		string				true	"Workspace ID"
//	@Param			sid	path		string				true	"Session ID"
//	@Success		200	{object}	proto.AgentSession
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/sessions/{sid} [get]
func (c *controllerV1) handleGetWorkspaceAgentSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	agentSession, err := c.backend.GetAgentSession(r.Context(), id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, agentSession)
}

// handlePostWorkspaceAgentSessionCancel cancels a running agent session.
//
//	@Summary		Cancel agent session
//	@Tags			agent
//	@Param			id	path	string	true	"Workspace ID"
//	@Param			sid	path	string	true	"Session ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/sessions/{sid}/cancel [post]
func (c *controllerV1) handlePostWorkspaceAgentSessionCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	if err := c.backend.CancelSession(id, sid); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetWorkspaceAgentSessionPromptQueued returns whether a queued prompt exists.
//
//	@Summary		Get queued prompt status
//	@Tags			agent
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Param			sid	path		string	true	"Session ID"
//	@Success		200	{object}	object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/sessions/{sid}/prompts/queued [get]
func (c *controllerV1) handleGetWorkspaceAgentSessionPromptQueued(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	queued, err := c.backend.QueuedPrompts(id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, queued)
}

// handlePostWorkspaceAgentSessionPromptClear clears the prompt queue for a session.
//
//	@Summary		Clear prompt queue
//	@Tags			agent
//	@Param			id	path	string	true	"Workspace ID"
//	@Param			sid	path	string	true	"Session ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/sessions/{sid}/prompts/clear [post]
func (c *controllerV1) handlePostWorkspaceAgentSessionPromptClear(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	if err := c.backend.ClearQueue(id, sid); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceAgentSessionSummarize summarizes a session.
//
//	@Summary		Summarize session
//	@Tags			agent
//	@Param			id	path	string	true	"Workspace ID"
//	@Param			sid	path	string	true	"Session ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/sessions/{sid}/summarize [post]
func (c *controllerV1) handlePostWorkspaceAgentSessionSummarize(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	if err := c.backend.SummarizeSession(r.Context(), id, sid); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetWorkspaceAgentSessionPromptList returns the list of queued prompts.
//
//	@Summary		List queued prompts
//	@Tags			agent
//	@Produce		json
//	@Param			id	path		string		true	"Workspace ID"
//	@Param			sid	path		string		true	"Session ID"
//	@Success		200	{array}		string
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/agent/sessions/{sid}/prompts/list [get]
func (c *controllerV1) handleGetWorkspaceAgentSessionPromptList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	prompts, err := c.backend.QueuedPromptsList(id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, prompts)
}

// handleGetWorkspaceAgentDefaultSmallModel returns the default small model for a provider.
//
//	@Summary		Get default small model
//	@Tags			agent
//	@Produce		json
//	@Param			id			path		string	true	"Workspace ID"
//	@Param			provider_id	query		string	false	"Provider ID"
//	@Success		200			{object}	object
//	@Failure		404			{object}	proto.Error
//	@Failure		500			{object}	proto.Error
//	@Router			/workspaces/{id}/agent/default-small-model [get]
func (c *controllerV1) handleGetWorkspaceAgentDefaultSmallModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	providerID := r.URL.Query().Get("provider_id")
	model, err := c.backend.GetDefaultSmallModel(id, providerID)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, model)
}

// handlePostWorkspacePermissionsGrant grants a permission request.
//
//	@Summary		Grant permission
//	@Tags			permissions
//	@Accept			json
//	@Param			id		path	string				true	"Workspace ID"
//	@Param			request	body	proto.PermissionGrant	true	"Permission grant"
//	@Success		200
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/permissions/grant [post]
func (c *controllerV1) handlePostWorkspacePermissionsGrant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.PermissionGrant
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.GrantPermission(id, req); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspacePermissionsSkip sets whether to skip permission prompts.
//
//	@Summary		Set skip permissions
//	@Tags			permissions
//	@Accept			json
//	@Param			id		path	string						true	"Workspace ID"
//	@Param			request	body	proto.PermissionSkipRequest	true	"Permission skip request"
//	@Success		200
//	@Failure		400	{object}	proto.Error
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/permissions/skip [post]
func (c *controllerV1) handlePostWorkspacePermissionsSkip(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req proto.PermissionSkipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	if err := c.backend.SetPermissionsSkip(id, req.Skip); err != nil {
		c.handleError(w, r, err)
		return
	}
}

// handleGetWorkspacePermissionsSkip returns whether permission prompts are skipped.
//
//	@Summary		Get skip permissions status
//	@Tags			permissions
//	@Produce		json
//	@Param			id	path		string						true	"Workspace ID"
//	@Success		200	{object}	proto.PermissionSkipRequest
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/permissions/skip [get]
func (c *controllerV1) handleGetWorkspacePermissionsSkip(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	skip, err := c.backend.GetPermissionsSkip(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.PermissionSkipRequest{Skip: skip})
}

// handleError maps backend errors to HTTP status codes and writes the
// JSON error response.
func (c *controllerV1) handleError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, backend.ErrWorkspaceNotFound):
		status = http.StatusNotFound
	case errors.Is(err, backend.ErrLSPClientNotFound):
		status = http.StatusNotFound
	case errors.Is(err, backend.ErrAgentNotInitialized):
		status = http.StatusBadRequest
	case errors.Is(err, backend.ErrPathRequired):
		status = http.StatusBadRequest
	case errors.Is(err, backend.ErrInvalidPermissionAction):
		status = http.StatusBadRequest
	case errors.Is(err, backend.ErrUnknownCommand):
		status = http.StatusBadRequest
	}
	c.server.logError(r, err.Error())
	jsonError(w, status, err.Error())
}

func jsonEncode(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(proto.Error{Message: message})
}
