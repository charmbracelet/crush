package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

type mockToolPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
	granted bool
	err     error
	lastReq permission.CreatePermissionRequest
}

func (m *mockToolPermissionService) Request(_ context.Context, req permission.CreatePermissionRequest) (bool, error) {
	m.lastReq = req
	return m.granted, m.err
}

func (m *mockToolPermissionService) EvaluateRequest(context.Context, permission.CreatePermissionRequest) (permission.EvaluationResult, error) {
	return permission.EvaluationResult{Decision: permission.EvaluationDecisionAllow}, nil
}

func (m *mockToolPermissionService) Prompt(context.Context, permission.PermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockToolPermissionService) Grant(permission.PermissionRequest)           {}
func (m *mockToolPermissionService) Deny(permission.PermissionRequest)            {}
func (m *mockToolPermissionService) GrantPersistent(permission.PermissionRequest) {}
func (m *mockToolPermissionService) HasPersistentPermission(permission.PermissionRequest) bool {
	return false
}
func (m *mockToolPermissionService) ClearPersistentPermissions(string)  {}
func (m *mockToolPermissionService) AutoApproveSession(string)          {}
func (m *mockToolPermissionService) SetSessionAutoApprove(string, bool) {}
func (m *mockToolPermissionService) IsSessionAutoApprove(string) bool   { return false }
func (m *mockToolPermissionService) SetSkipRequests(bool)               {}
func (m *mockToolPermissionService) SkipRequests() bool                 { return false }
func (m *mockToolPermissionService) SubscribeNotifications(context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

func TestRequestPermission_ReturnsToolErrorForPolicyDenied(t *testing.T) {
	t.Parallel()

	permissions := &mockToolPermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		err: permission.NewPermissionBlockedError(
			"This action was blocked by the Auto Mode safety policy.",
			"Reason: Request was denied by policy.",
		),
	}

	resp, err := RequestPermission(context.Background(), permissions, permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "This action was blocked by the Auto Mode safety policy.")
	require.Contains(t, resp.Content, "Reason: Request was denied by policy.")
}

func TestRequestPermission_ReturnsFatalErrorForUserDenied(t *testing.T) {
	t.Parallel()

	permissions := &mockToolPermissionService{
		Broker:  pubsub.NewBroker[permission.PermissionRequest](),
		granted: false,
	}

	resp, err := RequestPermission(context.Background(), permissions, permission.CreatePermissionRequest{})
	require.Nil(t, resp)
	require.Error(t, err)
	require.ErrorIs(t, err, permission.ErrorPermissionDenied)
}

func TestRequestPermission_PropagatesNonPermissionError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("permission service unavailable")
	permissions := &mockToolPermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		err:    expectedErr,
	}

	resp, err := RequestPermission(context.Background(), permissions, permission.CreatePermissionRequest{})
	require.Nil(t, resp)
	require.ErrorIs(t, err, expectedErr)
}

func TestResolveAuthoritySessionID_UsesParentSession(t *testing.T) {
	t.Parallel()

	svc := &mockPermissionSessionLookup{session: session.Session{ID: "child", ParentSessionID: "parent"}}
	ctx := context.WithValue(context.Background(), SessionServiceContextKey, svc)

	authoritySessionID := ResolveAuthoritySessionID(ctx, "child")
	require.Equal(t, "parent", authoritySessionID)
}

func TestResolveAuthoritySessionID_FallsBackToCurrentSession(t *testing.T) {
	t.Parallel()

	authoritySessionID := ResolveAuthoritySessionID(context.Background(), "child")
	require.Equal(t, "child", authoritySessionID)
}

func TestRequestPermission_SetsAuthoritySessionID(t *testing.T) {
	t.Parallel()

	permissions := &mockToolPermissionService{
		Broker:  pubsub.NewBroker[permission.PermissionRequest](),
		granted: true,
	}
	svc := &mockPermissionSessionLookup{session: session.Session{ID: "child", ParentSessionID: "parent"}}
	ctx := context.WithValue(context.Background(), SessionServiceContextKey, svc)

	resp, err := RequestPermission(ctx, permissions, permission.CreatePermissionRequest{SessionID: "child"})
	require.NoError(t, err)
	require.Nil(t, resp)
	require.Equal(t, "parent", permissions.lastReq.AuthoritySessionID)
}

type mockPermissionSessionLookup struct {
	session session.Session
	err     error
}

func (m *mockPermissionSessionLookup) Get(context.Context, string) (session.Session, error) {
	if m.err != nil {
		return session.Session{}, m.err
	}
	return m.session, nil
}
