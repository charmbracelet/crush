package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBashPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (m *mockBashPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockBashPermissionService) Grant(req permission.PermissionRequest) {}

func (m *mockBashPermissionService) Deny(req permission.PermissionRequest) {}

func (m *mockBashPermissionService) GrantPersistent(req permission.PermissionRequest) {}

func (m *mockBashPermissionService) AutoApproveSession(sessionID string) {}

func (m *mockBashPermissionService) SetSkipRequests(skip bool) {}

func (m *mockBashPermissionService) SkipRequests() bool {
	return false
}

func (m *mockBashPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

func TestIsDangerousCommand(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		dangerous bool
	}{
		{
			name:      "simple banned command - curl",
			command:   "curl https://example.com",
			dangerous: true,
		},
		{
			name:      "simple banned command - sudo",
			command:   "sudo apt-get update",
			dangerous: true,
		},
		{
			name:      "npm global install with --global",
			command:   "npm install --global typescript",
			dangerous: true,
		},
		{
			name:      "npm global install with -g",
			command:   "npm install -g typescript",
			dangerous: true,
		},
		{
			name:      "npm local install",
			command:   "npm install typescript",
			dangerous: false,
		},
		{
			name:      "go test with -exec",
			command:   "go test -exec ./malicious ./...",
			dangerous: true,
		},
		{
			name:      "go test without -exec",
			command:   "go test ./...",
			dangerous: false,
		},
		{
			name:      "safe command - ls",
			command:   "ls -la",
			dangerous: false,
		},
		{
			name:      "safe command - echo",
			command:   "echo hello",
			dangerous: false,
		},
		{
			name:      "safe command - git",
			command:   "git status",
			dangerous: false,
		},
		{
			name:      "pip install with --user",
			command:   "pip install --user requests",
			dangerous: true,
		},
		{
			name:      "pip install without --user",
			command:   "pip install requests",
			dangerous: false,
		},
		{
			name:      "brew install",
			command:   "brew install wget",
			dangerous: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDangerousCommand(tt.command)
			assert.Equal(t, tt.dangerous, result, "command: %s", tt.command)
		})
	}
}

func TestBashTool_DefaultAutoBackgroundThreshold(t *testing.T) {
	workingDir := t.TempDir()
	tool := newBashToolForTest(workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "default threshold",
		Command:     "echo done",
	})

	require.False(t, resp.IsError)
	var meta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.False(t, meta.Background)
	require.Empty(t, meta.ShellID)
	require.Contains(t, meta.Output, "done")
}

func TestBashTool_CustomAutoBackgroundThreshold(t *testing.T) {
	workingDir := t.TempDir()
	tool := newBashToolForTest(workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description:         "custom threshold",
		Command:             "sleep 1.5 && echo done",
		AutoBackgroundAfter: 1,
	})

	require.False(t, resp.IsError)
	var meta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.True(t, meta.Background)
	require.NotEmpty(t, meta.ShellID)
	require.Contains(t, resp.Content, "moved to background")

	bgManager := shell.GetBackgroundShellManager()
	require.NoError(t, bgManager.Kill(meta.ShellID))
}

func newBashToolForTest(workingDir string) fantasy.AgentTool {
	permissions := &mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	return NewBashTool(permissions, workingDir, attribution, "test-model")
}

func runBashTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params BashParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := fantasy.ToolCall{
		ID:    "test-call",
		Name:  BashToolName,
		Input: string(input),
	}

	resp, err := tool.Run(ctx, call)
	require.NoError(t, err)
	return resp
}
