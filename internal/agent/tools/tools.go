package tools

import (
	"context"

	"github.com/charmbracelet/crush/internal/permission"
)

type (
	sessionIDContextKey      string
	messageIDContextKey      string
	hookPermissionContextKey string
)

const (
	SessionIDContextKey      sessionIDContextKey      = "session_id"
	MessageIDContextKey      messageIDContextKey      = "message_id"
	HookPermissionContextKey hookPermissionContextKey = "hook_permission"
)

func GetSessionFromContext(ctx context.Context) string {
	sessionID := ctx.Value(SessionIDContextKey)
	if sessionID == nil {
		return ""
	}
	s, ok := sessionID.(string)
	if !ok {
		return ""
	}
	return s
}

func GetMessageFromContext(ctx context.Context) string {
	messageID := ctx.Value(MessageIDContextKey)
	if messageID == nil {
		return ""
	}
	s, ok := messageID.(string)
	if !ok {
		return ""
	}
	return s
}

// GetHookPermissionFromContext gets the hook permission decision from context.
// Returns: permission string ("approve" or "deny"), found bool
func GetHookPermissionFromContext(ctx context.Context) (string, bool) {
	permission := ctx.Value(HookPermissionContextKey)
	if permission == nil {
		return "", false
	}
	s, ok := permission.(string)
	if !ok {
		return "", false
	}
	return s, true
}

// SetHookPermissionInContext sets the hook permission decision in context.
func SetHookPermissionInContext(ctx context.Context, permission string) context.Context {
	return context.WithValue(ctx, HookPermissionContextKey, permission)
}

// CheckHookPermission checks if a hook has already made a permission decision.
// Returns true if execution should proceed, false if denied.
// If hook approved, skips the permission service call.
// If hook denied, returns ErrorPermissionDenied.
// If hook said "ask" or no decision, calls the permission service.
func CheckHookPermission(ctx context.Context, permissionService permission.Service, req permission.CreatePermissionRequest) (bool, error) {
	hookPerm, hasHookPerm := GetHookPermissionFromContext(ctx)

	if hasHookPerm {
		switch hookPerm {
		case "approve":
			// Hook auto-approved, skip permission check
			return true, nil
		case "deny":
			// Hook denied, return error
			return false, permission.ErrorPermissionDenied
		}
	}

	// No hook decision or hook said "ask", use normal permission flow
	granted := permissionService.Request(req)
	if !granted {
		return false, permission.ErrorPermissionDenied
	}
	return true, nil
}
