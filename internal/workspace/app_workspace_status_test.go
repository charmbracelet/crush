package workspace

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func newStatusTestWorkspace(t *testing.T) (*AppWorkspace, *app.App) {
	t.Helper()
	a := app.NewForTest(context.Background())
	t.Cleanup(a.ShutdownForTest)
	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	a.Sessions = session.NewService(db.New(conn), conn)
	return NewAppWorkspace(a, nil), a
}

func TestAppWorkspaceSessionStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, a := newStatusTestWorkspace(t)
	top, err := a.Sessions.Create(ctx, "top")
	require.NoError(t, err)
	other, err := a.Sessions.Create(ctx, "other")
	require.NoError(t, err)
	child, err := a.Sessions.CreateTaskSession(ctx, "child-1", top.ID, "child")
	require.NoError(t, err)

	require.Equal(t, SessionStatus{Known: true}, w.AgentSessionStatus(top.ID), "nothing pending, no coordinator")

	// Permission on the session itself: set, then cleared by Grant.
	reqCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = a.Permissions.Request(reqCtx, permission.CreatePermissionRequest{
			SessionID: top.ID, ToolCallID: "tc-1", ToolName: "bash", Action: "execute",
		})
	}()
	t.Cleanup(func() { cancel(); <-done })
	var req permission.PermissionRequest
	require.Eventually(t, func() bool {
		req, _ = a.Permissions.ActiveRequest()
		return req.ID != ""
	}, 10*time.Second, 5*time.Millisecond)
	require.True(t, w.AgentSessionStatus(top.ID).PendingPrompt)
	require.False(t, w.AgentSessionStatus(other.ID).PendingPrompt)
	require.True(t, a.Permissions.Grant(req))
	require.False(t, w.AgentSessionStatus(top.ID).PendingPrompt)

	// Question raised by a child session surfaces on the root.
	qDone := make(chan struct{})
	go func() {
		defer close(qDone)
		_, _ = a.Questions.Ask(ctx, question.Request{
			SessionID: child.ID,
			Questions: []question.Question{{Type: question.TypeFreeText, Text: "Proceed?", Description: "Tell me"}},
		})
	}()
	t.Cleanup(func() { a.Questions.Cancel(); <-qDone })
	require.Eventually(t, func() bool {
		_, ok := a.Questions.Pending()
		return ok
	}, 10*time.Second, 5*time.Millisecond)
	require.True(t, w.AgentSessionStatus(top.ID).PendingPrompt, "child's question surfaces on the root")
	require.False(t, w.AgentSessionStatus(other.ID).PendingPrompt)
}

func TestAppWorkspaceSessionStatusWithoutSessions(t *testing.T) {
	t.Parallel()
	a := app.NewForTest(context.Background())
	t.Cleanup(a.ShutdownForTest)
	w := NewAppWorkspace(a, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = a.Permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID: "s-other", ToolCallID: "tc-1", ToolName: "bash", Action: "execute",
		})
	}()
	t.Cleanup(func() { cancel(); <-done })
	require.Eventually(t, func() bool {
		_, ok := a.Permissions.ActiveRequest()
		return ok
	}, 10*time.Second, 5*time.Millisecond)

	require.NotPanics(t, func() {
		require.False(t, w.AgentSessionStatus("s-1").PendingPrompt, "another session's prompt, no session service")
	})
	require.True(t, w.AgentSessionStatus("s-other").PendingPrompt, "direct match still works")
}

func TestRootSessionID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, a := newStatusTestWorkspace(t)
	top, err := a.Sessions.Create(ctx, "top")
	require.NoError(t, err)
	child, err := a.Sessions.CreateTaskSession(ctx, "child-1", top.ID, "child")
	require.NoError(t, err)
	grand, err := a.Sessions.CreateTaskSession(ctx, "grand-1", child.ID, "grand")
	require.NoError(t, err)

	for _, id := range []string{top.ID, child.ID, grand.ID} {
		root, err := RootSessionID(ctx, w, id)
		require.NoError(t, err)
		require.Equal(t, top.ID, root)
	}

	// A chain MaxAncestryDepth hops deep still reaches the root; one
	// hop more stops short, exactly like the server-side resolver.
	chain := []string{top.ID}
	for i := range session.MaxAncestryDepth + 1 {
		next, err := a.Sessions.CreateTaskSession(ctx, fmt.Sprintf("chain-%d", i), chain[i], "chain")
		require.NoError(t, err)
		chain = append(chain, next.ID)
	}
	root, err := RootSessionID(ctx, w, chain[session.MaxAncestryDepth])
	require.NoError(t, err)
	require.Equal(t, top.ID, root)
	root, err = RootSessionID(ctx, w, chain[session.MaxAncestryDepth+1])
	require.NoError(t, err)
	require.Equal(t, chain[1], root, "beyond the depth limit the walk stops short")

	root, err = RootSessionID(ctx, w, "ghost")
	require.Error(t, err)
	require.Equal(t, "ghost", root, "input is returned alongside the error")
}
