package backend

import (
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
)

// GrantPermission grants, denies, or persistently grants a permission
// request.
func (b *Backend) GrantPermission(workspaceID string, req proto.PermissionGrant) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	perm := permission.PermissionRequest{
		ID:          req.Permission.ID,
		SessionID:   req.Permission.SessionID,
		ToolCallID:  req.Permission.ToolCallID,
		ToolName:    req.Permission.ToolName,
		Description: req.Permission.Description,
		Action:      req.Permission.Action,
		Params:      req.Permission.Params,
		Path:        req.Permission.Path,
	}

	switch req.Action {
	case proto.PermissionAllow:
		ws.Permissions.Grant(perm)
	case proto.PermissionAllowForSession:
		ws.Permissions.GrantPersistent(perm)
	case proto.PermissionDeny:
		ws.Permissions.Deny(perm)
	default:
		return ErrInvalidPermissionAction
	}
	return nil
}

// SetPermissionsSkip sets whether permission prompts are skipped.
func (b *Backend) SetPermissionsSkip(workspaceID string, skip bool) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	ws.Permissions.SetSkipRequests(skip)
	return nil
}

// GetPermissionsSkip returns whether permission prompts are skipped.
func (b *Backend) GetPermissionsSkip(workspaceID string) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}

	return ws.Permissions.SkipRequests(), nil
}
