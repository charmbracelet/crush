package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type memoryPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
	granted bool
	err     error
	req     permission.CreatePermissionRequest
}

func (m *memoryPermissionService) Request(_ context.Context, req permission.CreatePermissionRequest) (bool, error) {
	m.req = req
	return m.granted, m.err
}

func (m *memoryPermissionService) EvaluateRequest(context.Context, permission.CreatePermissionRequest) (permission.EvaluationResult, error) {
	return permission.EvaluationResult{Decision: permission.EvaluationDecisionAllow}, nil
}

func (m *memoryPermissionService) Prompt(context.Context, permission.PermissionRequest) (bool, error) {
	return true, nil
}

func (m *memoryPermissionService) Grant(permission.PermissionRequest)           {}
func (m *memoryPermissionService) Deny(permission.PermissionRequest)            {}
func (m *memoryPermissionService) GrantPersistent(permission.PermissionRequest) {}
func (m *memoryPermissionService) HasPersistentPermission(permission.PermissionRequest) bool {
	return false
}
func (m *memoryPermissionService) ClearPersistentPermissions(string)  {}
func (m *memoryPermissionService) AutoApproveSession(string)          {}
func (m *memoryPermissionService) SetSessionAutoApprove(string, bool) {}
func (m *memoryPermissionService) IsSessionAutoApprove(string) bool   { return false }
func (m *memoryPermissionService) SetSkipRequests(bool)               {}
func (m *memoryPermissionService) SkipRequests() bool                 { return false }
func (m *memoryPermissionService) SubscribeNotifications(context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

func runLongTermMemoryTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params LongTermMemoryParams) (fantasy.ToolResponse, error) {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	return tool.Run(ctx, fantasy.ToolCall{ID: "call-1", Name: LongTermMemoryToolName, Input: string(input)})
}

func newLongTermMemoryToolForTest(t *testing.T, permissions permission.Service) fantasy.AgentTool {
	t.Helper()
	memorySvc, err := memory.NewService(t.TempDir())
	require.NoError(t, err)
	return NewLongTermMemoryTool(memorySvc, permissions, "/workspace")
}

func TestLongTermMemoryToolStoreAndGet(t *testing.T) {
	t.Parallel()

	permissions := &memoryPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), granted: true}
	tool := newLongTermMemoryToolForTest(t, permissions)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session-1")

	storeResp, err := runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "store", Key: "goal", Value: "ship mvp"})
	require.NoError(t, err)
	require.False(t, storeResp.IsError)
	require.Contains(t, storeResp.Content, "Stored long-term memory key \"goal\".")
	require.Equal(t, "write", permissions.req.Action)

	getResp, err := runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "get", Key: "goal"})
	require.NoError(t, err)
	require.False(t, getResp.IsError)
	require.Contains(t, getResp.Content, "key=goal")
	require.Contains(t, getResp.Content, "value=ship mvp")
}

func TestLongTermMemoryToolDeleteAndList(t *testing.T) {
	t.Parallel()

	permissions := &memoryPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), granted: true}
	tool := newLongTermMemoryToolForTest(t, permissions)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session-1")

	_, err := runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "store", Key: "a", Value: "first"})
	require.NoError(t, err)
	_, err = runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "store", Key: "b", Value: "second"})
	require.NoError(t, err)

	listResp, err := runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "list", Limit: 10})
	require.NoError(t, err)
	require.False(t, listResp.IsError)
	require.Contains(t, listResp.Content, "Found 2 long-term memory entries")

	deleteResp, err := runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "delete", Key: "a"})
	require.NoError(t, err)
	require.False(t, deleteResp.IsError)
	require.Contains(t, deleteResp.Content, "Deleted long-term memory key \"a\".")
	require.Equal(t, "delete", permissions.req.Action)
}

func TestLongTermMemoryToolSearchAndErrors(t *testing.T) {
	t.Parallel()

	permissions := &memoryPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), granted: true}
	tool := newLongTermMemoryToolForTest(t, permissions)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session-1")

	_, err := runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "store", Key: "design", Value: "search index"})
	require.NoError(t, err)

	searchResp, err := runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "search", Query: "index"})
	require.NoError(t, err)
	require.False(t, searchResp.IsError)
	require.Contains(t, searchResp.Content, "key=design")

	invalidResp, err := runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "invalid"})
	require.NoError(t, err)
	require.True(t, invalidResp.IsError)
	require.Contains(t, invalidResp.Content, "action must be one of")

	missingResp, err := runLongTermMemoryTool(t, tool, ctx, LongTermMemoryParams{Action: "get", Key: "missing"})
	require.NoError(t, err)
	require.True(t, missingResp.IsError)
	require.Contains(t, missingResp.Content, "No long-term memory found for key \"missing\".")
}

func TestLongTermMemoryToolRequiresSessionForWrites(t *testing.T) {
	t.Parallel()

	permissions := &memoryPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), granted: true}
	tool := newLongTermMemoryToolForTest(t, permissions)

	_, err := runLongTermMemoryTool(t, tool, context.Background(), LongTermMemoryParams{Action: "store", Key: "k", Value: "v"})
	require.ErrorContains(t, err, "session ID is required")
}
