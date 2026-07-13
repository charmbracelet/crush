package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type recordingTmuxPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
	allow        bool
	requestCount int
	requests     []permission.CreatePermissionRequest
}

func (m *recordingTmuxPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	m.requestCount++
	m.requests = append(m.requests, req)
	return m.allow, nil
}

func (m *recordingTmuxPermissionService) Grant(req permission.PermissionRequest) bool { return true }

func (m *recordingTmuxPermissionService) Deny(req permission.PermissionRequest) bool { return true }

func (m *recordingTmuxPermissionService) GrantPersistent(req permission.PermissionRequest) bool {
	return true
}

func (m *recordingTmuxPermissionService) AutoApproveSession(sessionID string) {}

func (m *recordingTmuxPermissionService) SetSkipRequests(skip bool) {}

func (m *recordingTmuxPermissionService) SkipRequests() bool { return false }

func (m *recordingTmuxPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(chan pubsub.Event[permission.PermissionNotification])
}

func TestTmuxToolStartSendCaptureKill(t *testing.T) {
	requireTmuxForTest(t)

	workingDir := t.TempDir()
	session := fmt.Sprintf("crush-test-%d", time.Now().UnixNano())
	cleanupTmuxSession(t, session)

	perms := &recordingTmuxPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), allow: true}
	tool := NewTmuxTool(perms, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	startResp := runTmuxTool(t, tool, ctx, TmuxParams{Action: "start", Session: session, WorkingDir: workingDir})
	require.False(t, startResp.IsError, startResp.Content)
	require.Contains(t, startResp.Content, "Tmux session started")

	sendResp := runTmuxTool(t, tool, ctx, TmuxParams{Action: "send", Session: session, Input: "printf 'sent-from-tmux\\n'", DelayMillis: 500})
	require.False(t, sendResp.IsError, sendResp.Content)
	require.Contains(t, sendResp.Content, "sent-from-tmux")

	captureResp := runTmuxTool(t, tool, ctx, TmuxParams{Action: "capture", Session: session, Lines: 50})
	require.False(t, captureResp.IsError, captureResp.Content)
	require.Contains(t, captureResp.Content, "sent-from-tmux")

	killResp := runTmuxTool(t, tool, ctx, TmuxParams{Action: "kill", Session: session})
	require.False(t, killResp.IsError, killResp.Content)
	require.Contains(t, killResp.Content, "terminated")

	require.Equal(t, 3, perms.requestCount, "start, send, and kill should request permission")
	require.Equal(t, TmuxToolName, perms.requests[0].ToolName)
	require.Equal(t, "start", perms.requests[0].Action)
	require.Equal(t, session, perms.requests[0].Resource)
	require.Equal(t, "send", perms.requests[1].Action)
	require.Equal(t, "printf 'sent-from-tmux\\n'", perms.requests[1].Resource)
	require.Equal(t, "kill", perms.requests[2].Action)
}

func TestTmuxToolDeniedStartDoesNotCreateSession(t *testing.T) {
	requireTmuxForTest(t)

	workingDir := t.TempDir()
	session := fmt.Sprintf("crush-test-denied-%d", time.Now().UnixNano())
	cleanupTmuxSession(t, session)

	perms := &recordingTmuxPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), allow: false}
	tool := NewTmuxTool(perms, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runTmuxTool(t, tool, ctx, TmuxParams{Action: "start", Session: session, WorkingDir: workingDir})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "User denied permission")
	require.Equal(t, 1, perms.requestCount)

	listResp := runTmuxTool(t, tool, ctx, TmuxParams{Action: "list"})
	require.False(t, strings.Contains(listResp.Content, session), "denied session should not be created")
}

func runTmuxTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params TmuxParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  TmuxToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}

func requireTmuxForTest(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
}

func cleanupTmuxSession(t *testing.T, session string) {
	t.Helper()
	_ = exec.Command("tmux", "kill-session", "-t", session).Run()
	t.Cleanup(func() {
		_ = exec.Command("tmux", "kill-session", "-t", session).Run()
	})
}
