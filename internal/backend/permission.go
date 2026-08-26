package backend

import (
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
)

// GrantPermission grants, denies, or persistently grants a permission
// request. The returned bool reports whether this call resolved the
// pending request (true) or found it already resolved by a previous
// caller (false). A false return is not an error.
func (b *Backend) GrantPermission(workspaceID string, req proto.PermissionGrant) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
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
		return ws.Permissions.Grant(perm), nil
	case proto.PermissionAllowForSession:
		return ws.Permissions.GrantPersistent(perm), nil
	case proto.PermissionDeny:
		return ws.Permissions.Deny(perm), nil
	default:
		return false, ErrInvalidPermissionAction
	}
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

// AutoApproveSession auto-approves every permission request in a single
// session, without changing workspace-wide skip behavior.
func (b *Backend) AutoApproveSession(workspaceID, sessionID string) error {
	if sessionID == "" {
		return ErrSessionIDRequired
	}

	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	ws.Permissions.AutoApproveSession(sessionID)
	return nil
}

// RevokeAutoApproveSession drops the auto-approval hold a caller took
// with AutoApproveSession. A workspace outlives the client that took the
// hold, so a non-interactive run must give it back on the way out or
// every later permission request in that session — including ones an
// interactive client makes — is granted silently.
func (b *Backend) RevokeAutoApproveSession(workspaceID, sessionID string) error {
	if sessionID == "" {
		return ErrSessionIDRequired
	}

	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	ws.Permissions.RevokeAutoApproveSession(sessionID)
	return nil
}
