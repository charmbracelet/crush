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

func TestResolveBannedCommands(t *testing.T) {
	t.Parallel()

	t.Run("defaults remain blocked", func(t *testing.T) {
		t.Parallel()

		blocked, allowed := bashCommandLists(config.ToolBash{})
		require.Contains(t, blocked, "curl")
		require.Contains(t, blocked, "ssh")
		require.Empty(t, allowed)
	})

	t.Run("explicitly allowed commands are removed", func(t *testing.T) {
		t.Parallel()

		blocked, allowed := bashCommandLists(config.ToolBash{AllowedCommands: []string{"curl", "ssh", "unknown"}})
		require.NotContains(t, blocked, "curl")
		require.NotContains(t, blocked, "ssh")
		require.Contains(t, blocked, "wget")
		require.Equal(t, []string{"curl", "ssh"}, allowed)
	})

	t.Run("normalizes values and ignores non-overridable commands", func(t *testing.T) {
		t.Parallel()

		blocked, allowed := bashCommandLists(config.ToolBash{AllowedCommands: []string{" CURL ", "curl", "Alias", "go"}})
		require.NotContains(t, blocked, "curl")
		require.Contains(t, blocked, "alias")
		require.Equal(t, []string{"curl"}, allowed)
	})
}

func TestBlockFuncs_RespectAllowedCommands(t *testing.T) {
	t.Parallel()

	t.Run("default blockers reject blocked commands", func(t *testing.T) {
		t.Parallel()

		for _, blockFunc := range blockFuncs(config.ToolBash{}) {
			if blockFunc([]string{"curl", "https://example.com"}) {
				return
			}
		}
		t.Fatal("expected curl to be blocked by default")
	})

	t.Run("allowing a blocked package manager lifts its install blocker", func(t *testing.T) {
		t.Parallel()

		for _, blockFunc := range blockFuncs(config.ToolBash{AllowedCommands: []string{"apt"}}) {
			require.False(t, blockFunc([]string{"apt", "install", "ripgrep"}))
		}
	})

	t.Run("non blocked commands stay protected", func(t *testing.T) {
		t.Parallel()

		blocked := false
		for _, blockFunc := range blockFuncs(config.ToolBash{AllowedCommands: []string{"go"}}) {
			if blockFunc([]string{"go", "test", "-exec=bash"}) {
				blocked = true
				break
			}
		}
		require.True(t, blocked)
	})

	t.Run("alias remains blocked even if configured", func(t *testing.T) {
		t.Parallel()

		blocked := false
		for _, blockFunc := range blockFuncs(config.ToolBash{AllowedCommands: []string{"alias"}}) {
			if blockFunc([]string{"alias", "curl=echo nope"}) {
				blocked = true
				break
			}
		}
		require.True(t, blocked)
	})
}

func TestBashDescription_WarnsWhenDangerousCommandsAllowed(t *testing.T) {
	t.Parallel()

	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	description := bashDescription(attribution, "test-model", config.ToolBash{AllowedCommands: []string{"curl", "ssh", "go", "alias"}})

	require.Contains(t, description, "Allowed dangerous commands (curl, ssh)")
	require.Contains(t, description, "default blocked-command list")
	require.NotContains(t, description, "Allowed dangerous commands (curl, ssh, go")
}

func TestBashTool_BlockedCommandReturnsErrorOutput(t *testing.T) {
	workingDir := t.TempDir()
	tool := newBashToolForTest(workingDir, config.ToolBash{})
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "blocked command",
		Command:     "curl https://example.com",
	})

	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "command is not allowed for security reasons")
}

func TestBashTool_DefaultAutoBackgroundThreshold(t *testing.T) {
	workingDir := t.TempDir()
	tool := newBashToolForTest(workingDir, config.ToolBash{})
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
	tool := newBashToolForTest(workingDir, config.ToolBash{})
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

func newBashToolForTest(workingDir string, toolCfg config.ToolBash) fantasy.AgentTool {
	permissions := &mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	return NewBashTool(permissions, workingDir, attribution, "test-model", toolCfg)
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
