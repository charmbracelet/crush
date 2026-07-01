package testutil

import (
	"context"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// MockPermissionService implements permission.Service for testing. It always
// approves requests and returns normal mode.
type MockPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (m *MockPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	return true, nil
}

func (m *MockPermissionService) Grant(req permission.PermissionRequest) bool { return true }

func (m *MockPermissionService) Deny(req permission.PermissionRequest) bool { return true }

func (m *MockPermissionService) GrantPersistent(req permission.PermissionRequest) bool {
	return true
}

func (m *MockPermissionService) AutoApproveSession(sessionID string) {}

func (m *MockPermissionService) SetSkipRequests(skip bool) {}

func (m *MockPermissionService) SkipRequests() bool {
	return false
}

func (m *MockPermissionService) SetPermissionMode(mode permission.PermissionMode) {}

func (m *MockPermissionService) PermissionMode() permission.PermissionMode {
	return permission.PermissionModeNormal
}

func (m *MockPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

func (m *MockPermissionService) SubscribeModeChanges(ctx context.Context) <-chan pubsub.Event[permission.ModeChangedEvent] {
	return make(<-chan pubsub.Event[permission.ModeChangedEvent])
}
