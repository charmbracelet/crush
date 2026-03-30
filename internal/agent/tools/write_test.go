package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type mockWritePermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
	granted bool
	err     error
}

func (m *mockWritePermissionService) Request(context.Context, permission.CreatePermissionRequest) (bool, error) {
	return m.granted, m.err
}

func (m *mockWritePermissionService) EvaluateRequest(context.Context, permission.CreatePermissionRequest) (permission.EvaluationResult, error) {
	return permission.EvaluationResult{Decision: permission.EvaluationDecisionAllow}, nil
}

func (m *mockWritePermissionService) Prompt(context.Context, permission.PermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockWritePermissionService) Grant(permission.PermissionRequest)           {}
func (m *mockWritePermissionService) Deny(permission.PermissionRequest)            {}
func (m *mockWritePermissionService) GrantPersistent(permission.PermissionRequest) {}
func (m *mockWritePermissionService) HasPersistentPermission(permission.PermissionRequest) bool {
	return false
}
func (m *mockWritePermissionService) ClearPersistentPermissions(string)   {}
func (m *mockWritePermissionService) AutoApproveSession(string)           {}
func (m *mockWritePermissionService) SetSessionAutoApprove(string, bool)  {}
func (m *mockWritePermissionService) IsSessionAutoApprove(string) bool    { return false }
func (m *mockWritePermissionService) SetSkipRequests(bool)                {}
func (m *mockWritePermissionService) SkipRequests() bool                  { return false }
func (m *mockWritePermissionService) SubscribeNotifications(context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

type mockFileTracker struct{}

func (m *mockFileTracker) RecordRead(context.Context, string, string) {}
func (m *mockFileTracker) LastReadTime(context.Context, string, string) time.Time {
	return time.Time{}
}
func (m *mockFileTracker) ListReadFiles(context.Context, string) ([]string, error) { return nil, nil }

var _ filetracker.Service = (*mockFileTracker)(nil)
var _ history.Service = (*mockHistoryService)(nil)

func runWriteTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params WriteParams) (fantasy.ToolResponse, error) {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	return tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  WriteToolName,
		Input: string(input),
	})
}

func newWriteToolForTest(permissions permission.Service, workingDir string) fantasy.AgentTool {
	return NewWriteTool(nil, permissions, &mockHistoryService{Broker: pubsub.NewBroker[history.File]()}, &mockFileTracker{}, workingDir)
}

func TestWriteTool_ReturnsToolErrorForAutoModePolicyBlock(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	permissions := &mockWritePermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		err: permission.NewPermissionBlockedError(
			"This action was blocked by the Auto Mode safety policy.",
			"Reason: Write touches a file outside the approved scope.\nConfidence: medium",
		),
	}
	tool := newWriteToolForTest(permissions, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp, err := runWriteTool(t, tool, ctx, WriteParams{
		FilePath: "nested/blocked.txt",
		Content:  "denied",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "This action was blocked by the Auto Mode safety policy.")
	require.Contains(t, resp.Content, "Reason: Write touches a file outside the approved scope.")
	require.NoFileExists(t, filepath.Join(workingDir, "nested", "blocked.txt"))
	_, statErr := os.Stat(filepath.Join(workingDir, "nested"))
	require.True(t, os.IsNotExist(statErr))
}

func TestWriteTool_ReturnsFatalErrorForUserDeniedPermission(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	permissions := &mockWritePermissionService{
		Broker:  pubsub.NewBroker[permission.PermissionRequest](),
		granted: false,
	}
	tool := newWriteToolForTest(permissions, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	_, err := runWriteTool(t, tool, ctx, WriteParams{
		FilePath: "nested/blocked.txt",
		Content:  "denied",
	})
	require.Error(t, err)
	require.True(t, permission.IsPermissionError(err))
	require.NoFileExists(t, filepath.Join(workingDir, "nested", "blocked.txt"))
	_, statErr := os.Stat(filepath.Join(workingDir, "nested"))
	require.True(t, os.IsNotExist(statErr))
}

func TestWriteTool_WritesFileWhenPermissionGranted(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	permissions := &mockWritePermissionService{
		Broker:  pubsub.NewBroker[permission.PermissionRequest](),
		granted: true,
	}
	tool := newWriteToolForTest(permissions, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp, err := runWriteTool(t, tool, ctx, WriteParams{
		FilePath: "allowed.txt",
		Content:  "hello",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	data, readErr := os.ReadFile(filepath.Join(workingDir, "allowed.txt"))
	require.NoError(t, readErr)
	require.Equal(t, "hello", string(data))
}
