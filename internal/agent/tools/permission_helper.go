package tools

import (
	"context"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
)

// RequestPermission runs the permission flow and converts eligible Auto Mode
// policy blocks into non-fatal tool error responses so the agent can continue.
func RequestPermission(ctx context.Context, permissions permission.Service, req permission.CreatePermissionRequest) (*fantasy.ToolResponse, error) {
	if req.AuthoritySessionID == "" {
		req.AuthoritySessionID = ResolveAuthoritySessionID(ctx, req.SessionID)
	}

	granted, err := permissions.Request(ctx, req)
	if err != nil {
		permissionErr, ok := permission.AsPermissionError(err)
		if ok && shouldReturnPermissionToolError(permissionErr) {
			response := fantasy.NewTextErrorResponse(formatPermissionToolError(permissionErr))
			return &response, nil
		}
		return nil, err
	}
	if !granted {
		return nil, permission.ErrorPermissionDenied
	}
	return nil, nil
}

func shouldReturnPermissionToolError(permissionErr *permission.PermissionError) bool {
	return permissionErr != nil && permissionErr.Kind == permission.PermissionErrorKindPolicyDenied
}

func ResolveAuthoritySessionID(ctx context.Context, sessionID string) string {
	authoritySessionID := sessionID
	sessionSvc := GetSessionServiceFromContext(ctx)
	if sessionSvc == nil || sessionID == "" {
		return authoritySessionID
	}

	sess, err := sessionSvc.Get(ctx, sessionID)
	if err != nil {
		return authoritySessionID
	}
	if sess.ParentSessionID != "" {
		return sess.ParentSessionID
	}
	return authoritySessionID
}

func formatPermissionToolError(permissionErr *permission.PermissionError) string {
	lines := []string{"This action was blocked by the Auto Mode safety policy."}
	if permissionErr == nil {
		return lines[0]
	}

	details := strings.TrimSpace(permissionErr.Details)
	if details != "" {
		lines = append(lines, details)
		return strings.Join(lines, "\n")
	}

	reason := strings.TrimSpace(permissionErr.Message)
	if reason != "" && !strings.EqualFold(reason, lines[0]) {
		lines = append(lines, "Reason: "+reason)
	}
	return strings.Join(lines, "\n")
}
