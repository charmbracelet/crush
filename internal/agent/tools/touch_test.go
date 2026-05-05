package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// recordingPermissionService captures permission requests and answers them
// according to a configurable response.
type recordingPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
	requests []permission.CreatePermissionRequest
	grant    bool
}

func (m *recordingPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	m.requests = append(m.requests, req)
	return m.grant, nil
}

func (m *recordingPermissionService) Grant(req permission.PermissionRequest)           {}
func (m *recordingPermissionService) Deny(req permission.PermissionRequest)            {}
func (m *recordingPermissionService) GrantPersistent(req permission.PermissionRequest) {}
func (m *recordingPermissionService) AutoApproveSession(sessionID string)              {}
func (m *recordingPermissionService) SetSkipRequests(skip bool)                        {}
func (m *recordingPermissionService) SkipRequests() bool                               { return false }
func (m *recordingPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

type mockFileTrackerService struct{}

func (m mockFileTrackerService) RecordRead(ctx context.Context, sessionID, path string) {}

func (m mockFileTrackerService) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	return time.Now()
}

func (m mockFileTrackerService) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return nil, nil
}

func TestTouchToolCreatesEmptyFile(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	tool := NewTouchTool(nil, &mockPermissionService{}, &mockHistoryService{}, mockFileTrackerService{}, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runTouchTool(t, tool, ctx, TouchParams{FilePath: "nested/empty.txt"})
	require.False(t, resp.IsError)

	filePath := filepath.Join(workingDir, "nested", "empty.txt")
	info, err := os.Stat(filePath)
	require.NoError(t, err)
	require.False(t, info.IsDir())
	require.Zero(t, info.Size())
}

func TestTouchToolRefusesExistingFile(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "existing.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

	tool := NewTouchTool(nil, &mockPermissionService{}, &mockHistoryService{}, mockFileTrackerService{}, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runTouchTool(t, tool, ctx, TouchParams{FilePath: "existing.txt"})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "File already exists")

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "content", string(content))
}

func TestTouchToolStaysInsideWorkingDir(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	perms := &recordingPermissionService{grant: true}
	tool := NewTouchTool(nil, perms, &mockHistoryService{}, mockFileTrackerService{}, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runTouchTool(t, tool, ctx, TouchParams{FilePath: "inside.txt"})
	require.False(t, resp.IsError)

	for _, req := range perms.requests {
		require.NotContains(t, req.Description, "outside working directory",
			"inside-workingDir touch should not trigger an outside-workingDir permission prompt")
	}

	_, err := os.Stat(filepath.Join(workingDir, "inside.txt"))
	require.NoError(t, err)
}

func TestTouchToolOutsideWorkingDirRequiresPermission(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	workingDir := filepath.Join(parent, "wd")
	require.NoError(t, os.MkdirAll(workingDir, 0o755))

	// Denied: file outside workingDir must not be created.
	deny := &recordingPermissionService{grant: false}
	tool := NewTouchTool(nil, deny, &mockHistoryService{}, mockFileTrackerService{}, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runTouchTool(t, tool, ctx, TouchParams{FilePath: "../escape.txt"})
	require.True(t, resp.IsError)

	require.Len(t, deny.requests, 1)
	require.True(t, strings.Contains(deny.requests[0].Description, "outside working directory"),
		"expected outside-working-directory permission prompt, got %q", deny.requests[0].Description)

	_, err := os.Stat(filepath.Join(parent, "escape.txt"))
	require.True(t, os.IsNotExist(err), "denied permission should not create the file")

	// Granted: same path now succeeds.
	grant := &recordingPermissionService{grant: true}
	tool = NewTouchTool(nil, grant, &mockHistoryService{}, mockFileTrackerService{}, workingDir)
	resp = runTouchTool(t, tool, ctx, TouchParams{FilePath: "../escape.txt"})
	require.False(t, resp.IsError)
	require.GreaterOrEqual(t, len(grant.requests), 1)
	require.Contains(t, grant.requests[0].Description, "outside working directory")

	_, err = os.Stat(filepath.Join(parent, "escape.txt"))
	require.NoError(t, err)
}

func TestWriteToolEmptyContentPointsToTouch(t *testing.T) {
	t.Parallel()

	tool := NewWriteTool(nil, nil, nil, nil, t.TempDir())

	input, err := json.Marshal(WriteParams{FilePath: "empty.txt"})
	require.NoError(t, err)

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "test-call",
		Name:  WriteToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Equal(t, `content is required. use the "touch" tool to create an empty file`, resp.Content)
}

func runTouchTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params TouchParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  TouchToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}
