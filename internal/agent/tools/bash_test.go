package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/stretchr/testify/require"
)

type mockBashPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (m *mockBashPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockBashPermissionService) Grant(req permission.PermissionRequest) bool { return true }

func (m *mockBashPermissionService) Deny(req permission.PermissionRequest) bool { return true }

func (m *mockBashPermissionService) GrantPersistent(req permission.PermissionRequest) bool {
	return true
}

func (m *mockBashPermissionService) AutoApproveSession(sessionID string) {}

func (m *mockBashPermissionService) SetSkipRequests(skip bool) {}

func (m *mockBashPermissionService) SkipRequests() bool {
	return false
}

func (m *mockBashPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
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

type recordingPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
	requestCount int
	allow        bool
}

func (m *recordingPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	m.requestCount++
	return m.allow, nil
}

func (m *recordingPermissionService) Grant(req permission.PermissionRequest) bool { return true }

func (m *recordingPermissionService) Deny(req permission.PermissionRequest) bool { return true }

func (m *recordingPermissionService) GrantPersistent(req permission.PermissionRequest) bool {
	return true
}

func (m *recordingPermissionService) AutoApproveSession(sessionID string) {}

func (m *recordingPermissionService) SetSkipRequests(skip bool) {}

func (m *recordingPermissionService) SkipRequests() bool {
	return false
}

func (m *recordingPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

// fakeTerminalService records interactive terminal requests and returns a
// canned result without needing a real PTY.
type fakeTerminalService struct {
	result   terminal.Result
	err      error
	requests []terminal.Request
}

func (f *fakeTerminalService) Subscribe(context.Context) <-chan pubsub.Event[terminal.Request] {
	return make(<-chan pubsub.Event[terminal.Request])
}

func (f *fakeTerminalService) SubscribeNotifications(context.Context) <-chan pubsub.Event[terminal.Notification] {
	return make(<-chan pubsub.Event[terminal.Notification])
}

func (f *fakeTerminalService) Run(_ context.Context, req terminal.Request) (terminal.Result, error) {
	f.requests = append(f.requests, req)
	return f.result, f.err
}

func (f *fakeTerminalService) Complete(terminal.Result) bool { return true }

func (f *fakeTerminalService) Cancel() bool { return true }

func (f *fakeTerminalService) SetAvailable(bool) {}

func TestBashTool_Interactive(t *testing.T) {
	workingDir := t.TempDir()
	permissions := &mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	terminals := &fakeTerminalService{
		result: terminal.Result{Output: "Select an account:\n> personal", WorkingDir: workingDir},
	}
	tool := NewBashTool(permissions, terminals, workingDir, workingDir, attribution, "test-model")
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "login",
		Command:     "gh auth login",
		Interactive: true,
	})

	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Select an account")
	require.Len(t, terminals.requests, 1)
	require.Equal(t, "gh auth login", terminals.requests[0].Command)
	require.Equal(t, workingDir, terminals.requests[0].WorkingDir)
	require.Equal(t, "test-session", terminals.requests[0].SessionID)

	var meta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.True(t, meta.Interactive)
	require.Equal(t, workingDir, meta.WorkingDirectory)
}

func TestBashTool_InteractiveReportsExitCodeAndTermination(t *testing.T) {
	workingDir := t.TempDir()
	permissions := &mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	terminals := &fakeTerminalService{
		result: terminal.Result{Output: "partial", ExitCode: 130, WorkingDir: workingDir, Terminated: true},
	}
	tool := NewBashTool(permissions, terminals, workingDir, workingDir, attribution, "test-model")
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "interactive",
		Command:     "nano file.txt",
		Interactive: true,
	})

	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Exit code 130")
	require.Contains(t, resp.Content, "user ended the interactive session")
}

func TestBashTool_InteractiveUnavailable(t *testing.T) {
	workingDir := t.TempDir()
	tool := newBashToolForTest(workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "interactive",
		Command:     "gh auth login",
		Interactive: true,
	})

	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "interactive mode is unavailable")
}

func TestBashTool_InteractiveCancelled(t *testing.T) {
	workingDir := t.TempDir()
	permissions := &mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	terminals := &fakeTerminalService{err: terminal.ErrCancelled}
	tool := NewBashTool(permissions, terminals, workingDir, workingDir, attribution, "test-model")
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "interactive",
		Command:     "gh auth login",
		Interactive: true,
	})

	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "cancelled")
}

func TestBashTool_InteractiveCancelledContext(t *testing.T) {
	workingDir := t.TempDir()
	permissions := &mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	terminals := &fakeTerminalService{err: context.Canceled}
	tool := NewBashTool(permissions, terminals, workingDir, workingDir, attribution, "test-model")
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	_, err := runBashToolRaw(t, tool, ctx, BashParams{
		Description: "interactive",
		Command:     "gh auth login",
		Interactive: true,
	})
	require.True(t, errors.Is(err, context.Canceled))
}

func newBashToolForTest(workingDir string) fantasy.AgentTool {
	permissions := &mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	return NewBashTool(permissions, nil, workingDir, workingDir, attribution, "test-model")
}

func newBashToolWithRecordingPerms(workingDir string, allow bool) (fantasy.AgentTool, *recordingPermissionService) {
	perms := &recordingPermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		allow:  allow,
	}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	return NewBashTool(perms, nil, workingDir, workingDir, attribution, "test-model"), perms
}

func TestBashTool_ChainedCommandsRequirePermission(t *testing.T) {
	workingDir := t.TempDir()
	tool, perms := newBashToolWithRecordingPerms(workingDir, true)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	// ls && echo should trigger permission check.
	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "chained ls",
		Command:     "ls && echo done",
	})

	require.False(t, resp.IsError)
	require.Equal(t, 1, perms.requestCount, "chained command should trigger permission request")

	// Plain ls should NOT trigger permission check.
	perms.requestCount = 0
	resp = runBashTool(t, tool, ctx, BashParams{
		Description: "plain ls",
		Command:     "ls -la",
	})

	require.False(t, resp.IsError)
	require.Equal(t, 0, perms.requestCount, "plain ls should not trigger permission request")
}

func TestBashTool_ChainedCommandsDenied(t *testing.T) {
	workingDir := t.TempDir()
	tool, perms := newBashToolWithRecordingPerms(workingDir, false)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "chained ls denied",
		Command:     "ls && rm -rf /",
	})

	require.Equal(t, 1, perms.requestCount)
	require.Contains(t, resp.Content, "User denied permission")
}

func runBashTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params BashParams) fantasy.ToolResponse {
	t.Helper()

	resp, err := runBashToolRaw(t, tool, ctx, params)
	require.NoError(t, err)
	return resp
}

func runBashToolRaw(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params BashParams) (fantasy.ToolResponse, error) {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := fantasy.ToolCall{
		ID:    "test-call",
		Name:  BashToolName,
		Input: string(input),
	}

	return tool.Run(ctx, call)
}

func TestTruncateOutputValidUTF8(t *testing.T) {
	t.Parallel()
	// CJK characters are 2 cells wide; this string is far wider than
	// MaxOutputLength so TruncateOutput must truncate it.
	content := strings.Repeat("你好世界", MaxOutputLength)

	out := TruncateOutput(content, t.TempDir())
	require.True(t, utf8.ValidString(out), "truncated output must stay valid UTF-8")
	require.Contains(t, out, "lines truncated")
}

func TestTruncateOutputShortContent(t *testing.T) {
	t.Parallel()
	content := "short output"
	require.Equal(t, content, TruncateOutput(content, t.TempDir()))
}

func TestTruncateOutputEmoji(t *testing.T) {
	t.Parallel()
	// Emoji with ZWJ sequences should not be split.
	content := strings.Repeat("👨‍👩‍👧‍👦", MaxOutputLength)

	out := TruncateOutput(content, t.TempDir())
	require.True(t, utf8.ValidString(out), "truncated output must stay valid UTF-8")
	require.Contains(t, out, "lines truncated")
}
