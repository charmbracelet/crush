package server

import (
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/crush/internal/fork"
)

// handleGetWorkspaceSnapshotsEnabled returns whether snapshots are enabled.
//
//	@Summary		Check if snapshots are enabled
//	@Tags			snapshots
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{object}	object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/snapshots/enabled [get]
func (c *controllerV1) handleGetWorkspaceSnapshotsEnabled(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	enabled, err := c.backend.SnapshotsEnabled(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, map[string]bool{"enabled": enabled})
}

// handleGetWorkspaceSnapshots lists snapshots for a session.
//
//	@Summary		List snapshots
//	@Tags			snapshots
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Param			sid	path		string	true	"Session ID"
//	@Success		200	{array}		object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/snapshots [get]
func (c *controllerV1) handleGetWorkspaceSnapshots(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	snapshots, err := c.backend.ListSnapshots(r.Context(), id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, snapshots)
}

// handleGetWorkspaceSnapshot retrieves a specific snapshot.
//
//	@Summary		Get snapshot
//	@Tags			snapshots
//	@Produce		json
//	@Param			id		path		string	true	"Workspace ID"
//	@Param			snapid	path		string	true	"Snapshot ID"
//	@Success		200		{object}	object
//	@Failure		404		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces/{id}/snapshots/{snapid} [get]
func (c *controllerV1) handleGetWorkspaceSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	snapid := r.PathValue("snapid")
	snapshot, err := c.backend.GetSnapshot(r.Context(), id, snapid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, snapshot)
}

// handleGetWorkspaceSnapshotByMessage retrieves a snapshot by message ID.
//
//	@Summary		Get snapshot by message
//	@Tags			snapshots
//	@Produce		json
//	@Param			id		path		string	true	"Workspace ID"
//	@Param			msgid	path		string	true	"Message ID"
//	@Success		200		{object}	object
//	@Failure		404		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces/{id}/messages/{msgid}/snapshot [get]
func (c *controllerV1) handleGetWorkspaceSnapshotByMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	msgid := r.PathValue("msgid")
	snapshot, err := c.backend.GetSnapshotByMessage(r.Context(), id, msgid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, snapshot)
}

// handlePostWorkspaceSnapshotRestore restores a snapshot.
//
//	@Summary		Restore snapshot
//	@Tags			snapshots
//	@Param			id		path	string	true	"Workspace ID"
//	@Param			snapid	path	string	true	"Snapshot ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/snapshots/{snapid}/restore [post]
func (c *controllerV1) handlePostWorkspaceSnapshotRestore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	snapid := r.PathValue("snapid")
	if err := c.backend.RestoreSnapshot(r.Context(), id, snapid); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetWorkspaceSnapshotDiff returns diff from current filesystem to snapshot.
//
//	@Summary		Get snapshot diff
//	@Tags			snapshots
//	@Produce		json
//	@Param			id		path		string	true	"Workspace ID"
//	@Param			snapid	path		string	true	"Snapshot ID"
//	@Success		200		{object}	object
//	@Failure		404		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces/{id}/snapshots/{snapid}/diff [get]
func (c *controllerV1) handleGetWorkspaceSnapshotDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	snapid := r.PathValue("snapid")
	diff, err := c.backend.DiffFromCurrentSnapshot(r.Context(), id, snapid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, map[string]string{"diff": diff})
}

// handlePostWorkspaceSnapshotGC runs garbage collection on snapshots.
//
//	@Summary		Run snapshot garbage collection
//	@Tags			snapshots
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{object}	object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/snapshots/gc [post]
func (c *controllerV1) handlePostWorkspaceSnapshotGC(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	freed, err := c.backend.SnapshotGC(r.Context(), id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, map[string]int64{"bytes_freed": freed})
}

// handleGetWorkspaceSnapshotStats returns snapshot storage statistics.
//
//	@Summary		Get snapshot stats
//	@Tags			snapshots
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{object}	object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/snapshots/stats [get]
func (c *controllerV1) handleGetWorkspaceSnapshotStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stats, err := c.backend.SnapshotStats(r.Context(), id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, stats)
}

// handleGetWorkspaceWorktreesEnabled returns whether worktrees are enabled.
//
//	@Summary		Check if worktrees are enabled
//	@Tags			worktrees
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{object}	object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/worktrees/enabled [get]
func (c *controllerV1) handleGetWorkspaceWorktreesEnabled(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	enabled, err := c.backend.WorktreesEnabled(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, map[string]bool{"enabled": enabled})
}

// handleGetWorkspaceWorktrees lists worktrees for a session.
//
//	@Summary		List worktrees
//	@Tags			worktrees
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Param			sid	path		string	true	"Session ID"
//	@Success		200	{array}		object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/worktrees [get]
func (c *controllerV1) handleGetWorkspaceWorktrees(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	worktrees, err := c.backend.ListWorktrees(r.Context(), id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, worktrees)
}

// handleGetAllWorkspaceWorktrees lists all worktrees for a workspace.
//
//	@Summary		List all worktrees
//	@Tags			worktrees
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{array}		object
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/worktrees [get]
func (c *controllerV1) handleGetAllWorkspaceWorktrees(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	worktrees, err := c.backend.ListAllWorktrees(r.Context(), id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, worktrees)
}

// handleGetWorkspaceWorktree retrieves a specific worktree.
//
//	@Summary		Get worktree
//	@Tags			worktrees
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Param			wtid	path		string	true	"Worktree ID"
//	@Success		200		{object}	object
//	@Failure		404		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces/{id}/worktrees/{wtid} [get]
func (c *controllerV1) handleGetWorkspaceWorktree(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wtid := r.PathValue("wtid")
	wt, err := c.backend.GetWorktree(r.Context(), id, wtid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, wt)
}

// handleGetWorkspaceActiveWorktree retrieves the active worktree for a session.
//
//	@Summary		Get active worktree
//	@Tags			worktrees
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Param			sid	path		string	true	"Session ID"
//	@Success		200		{object}	object
//	@Failure		404		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/worktrees/active [get]
func (c *controllerV1) handleGetWorkspaceActiveWorktree(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	wt, err := c.backend.GetActiveWorktree(r.Context(), id, sid)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, wt)
}

// CreateWorktreeRequest is the request body for creating a worktree.
type CreateWorktreeRequest struct {
	Name           string `json:"name"`
	FromSnapshotID string `json:"from_snapshot_id"`
}

// handlePostWorkspaceWorktree creates a new worktree.
//
//	@Summary		Create worktree
//	@Tags			worktrees
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string	true	"Workspace ID"
//	@Param			sid		path		string	true	"Session ID"
//	@Param			request	body		CreateWorktreeRequest	true	"Worktree creation params"
//	@Success		200		{object}	object
//	@Failure		400		{object}	proto.Error
//	@Failure		404		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/worktrees [post]
func (c *controllerV1) handlePostWorkspaceWorktree(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")

	var req CreateWorktreeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	wt, err := c.backend.CreateWorktree(r.Context(), id, sid, req.Name, req.FromSnapshotID)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, wt)
}

// handlePostWorkspaceWorktreeSwitch switches to a different worktree.
//
//	@Summary		Switch worktree
//	@Tags			worktrees
//	@Param			id		path	string	true	"Workspace ID"
//	@Param			sid		path	string	true	"Session ID"
//	@Param			wtid	path	string	true	"Worktree ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/sessions/{sid}/worktrees/{wtid}/switch [post]
func (c *controllerV1) handlePostWorkspaceWorktreeSwitch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	wtid := r.PathValue("wtid")
	if err := c.backend.SwitchWorktree(r.Context(), id, sid, wtid); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleDeleteWorkspaceWorktree deletes a worktree.
//
//	@Summary		Delete worktree
//	@Tags			worktrees
//	@Param			id		path	string	true	"Workspace ID"
//	@Param			wtid	path	string	true	"Worktree ID"
//	@Success		200
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/worktrees/{wtid} [delete]
func (c *controllerV1) handleDeleteWorkspaceWorktree(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wtid := r.PathValue("wtid")
	if err := c.backend.DeleteWorktree(r.Context(), id, wtid); err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePostWorkspaceFork creates a fork of a conversation.
//
//	@Summary		Fork conversation
//	@Tags			forks
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string	true	"Workspace ID"
//	@Param			request	body		fork.ForkParams	true	"Fork params"
//	@Success		200		{object}	object
//	@Failure		400		{object}	proto.Error
//	@Failure		404		{object}	proto.Error
//	@Failure		500		{object}	proto.Error
//	@Router			/workspaces/{id}/fork [post]
func (c *controllerV1) handlePostWorkspaceFork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var params fork.ForkParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		c.server.logError(r, "Failed to decode request", "error", err)
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}

	result, err := c.backend.ForkConversation(r.Context(), id, params)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}
