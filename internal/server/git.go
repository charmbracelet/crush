package server

import (
	"net/http"

	"github.com/charmbracelet/crush/internal/proto"
)

// handleGetWorkspaceGitBranch returns the current Git branch of the
// workspace's working directory.
//
//	@Summary		Get current Git branch
//	@Tags			git
//	@Param			id	path	string	true	"Workspace ID"
//	@Success		200	{object}	proto.GitBranchResponse
//	@Failure		404	{object}	proto.Error
//	@Failure		500	{object}	proto.Error
//	@Router			/workspaces/{id}/git/branch [get]
func (c *controllerV1) handleGetWorkspaceGitBranch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	branch, err := c.backend.GitBranch(id)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, proto.GitBranchResponse{Branch: branch})
}
