package backend

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// requestPermission raises a pending permission request for sessionID and
// returns it once it is active. The waiter is cancelled and awaited on
// cleanup if the test did not resolve it.
func requestPermission(t *testing.T, a *app.App, sessionID string) permission.PermissionRequest {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = a.Permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID:  sessionID,
			ToolCallID: "tc-" + sessionID,
			ToolName:   "bash",
			Action:     "execute",
		})
	}()
	t.Cleanup(func() { cancel(); <-done })

	var active permission.PermissionRequest
	require.Eventually(t, func() bool {
		active, _ = a.Permissions.ActiveRequest()
		return active.ID != ""
	}, 10*time.Second, 5*time.Millisecond, "request must be pending")
	return active
}

func newSessionStatusBackend(t *testing.T) (*Backend, *Workspace, *app.App) {
	t.Helper()
	b, ws, a := newPromptsTestBackend(t)
	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	a.Sessions = session.NewService(db.New(conn), conn)
	return b, ws, a
}

func TestGetAgentSessionReportsPendingPermission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b, ws, a := newSessionStatusBackend(t)
	top, err := a.Sessions.Create(ctx, "top")
	require.NoError(t, err)

	got, err := b.GetAgentSession(ctx, ws.ID, top.ID)
	require.NoError(t, err)
	require.False(t, got.PendingPrompt, "nothing pending yet")

	req := requestPermission(t, a, top.ID)
	got, err = b.GetAgentSession(ctx, ws.ID, top.ID)
	require.NoError(t, err)
	require.True(t, got.PendingPrompt)

	require.True(t, a.Permissions.Grant(req))
	got, err = b.GetAgentSession(ctx, ws.ID, top.ID)
	require.NoError(t, err)
	require.False(t, got.PendingPrompt, "granted request is no longer pending")
}

func TestGetAgentSessionPendingFromChildSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b, ws, a := newSessionStatusBackend(t)
	top, err := a.Sessions.Create(ctx, "top")
	require.NoError(t, err)
	other, err := a.Sessions.Create(ctx, "other")
	require.NoError(t, err)
	child, err := a.Sessions.CreateTaskSession(ctx, "child-1", top.ID, "child")
	require.NoError(t, err)

	requestPermission(t, a, child.ID)

	got, err := b.GetAgentSession(ctx, ws.ID, top.ID)
	require.NoError(t, err)
	require.True(t, got.PendingPrompt, "child's prompt surfaces on the root")

	got, err = b.GetAgentSession(ctx, ws.ID, other.ID)
	require.NoError(t, err)
	require.False(t, got.PendingPrompt, "unrelated session stays clean")
}
