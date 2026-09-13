package acp

import (
	"context"

	"github.com/google/uuid"

	"github.com/charmbracelet/crush/internal/pubsub"

	"github.com/charmbracelet/crush/internal/permission"
)

// acpPermissionService wraps a permission.Service to route requests
// through the ACP permission bridge instead of the UI.
type acpPermissionService struct {
	real   permission.Service
	bridge *permissionBridge
}

// newACPPermissionService creates a wrapper that uses the ACP bridge.
func newACPPermissionService(real permission.Service, bridge *permissionBridge) permission.Service {
	return &acpPermissionService{
		real:   real,
		bridge: bridge,
	}
}

// Request routes through the ACP bridge for interactive permission handling.
func (s *acpPermissionService) Request(ctx context.Context, opts permission.CreatePermissionRequest) (bool, error) {
	req := permission.PermissionRequest{
		ID:          uuid.New().String(),
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
		Path:        opts.Path,
	}
	return s.bridge.CheckPermission(ctx, req)
}

// Grant delegates to the real service.
func (s *acpPermissionService) Grant(p permission.PermissionRequest) bool {
	return s.real.Grant(p)
}

// GrantPersistent delegates to the real service.
func (s *acpPermissionService) GrantPersistent(p permission.PermissionRequest) bool {
	return s.real.GrantPersistent(p)
}

// Deny delegates to the real service.
func (s *acpPermissionService) Deny(p permission.PermissionRequest) bool {
	return s.real.Deny(p)
}

// Subscribe delegates to the real service.
func (s *acpPermissionService) Subscribe(ctx context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	return s.real.Subscribe(ctx)
}

// SubscribeNotifications delegates to the real service.
func (s *acpPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return s.real.SubscribeNotifications(ctx)
}

// AutoApproveSession delegates to the real service.
func (s *acpPermissionService) AutoApproveSession(sessionID string) {
	s.real.AutoApproveSession(sessionID)
}

// SetSkipRequests delegates to the real service.
func (s *acpPermissionService) SetSkipRequests(skip bool) {
	s.real.SetSkipRequests(skip)
}

// SkipRequests delegates to the real service.
func (s *acpPermissionService) SkipRequests() bool {
	return s.real.SkipRequests()
}
