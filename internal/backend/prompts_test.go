package backend

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// newPromptsTestBackend builds a backend with a workspace backed by a
// minimal in-process app (live event broker, real permission and
// question services, no agent machinery).
func newPromptsTestBackend(t *testing.T) (*Backend, *Workspace, *app.App) {
	t.Helper()
	ctx := context.Background()
	a := app.NewForTest(ctx)
	t.Cleanup(a.ShutdownForTest)
	b := New(ctx, nil, nil)
	ws := &Workspace{ID: uuid.New().String(), Path: t.TempDir(), App: a}
	InsertWorkspaceForTest(b, ws)
	return b, ws, a
}

// drainQuiet discards events for the given window. It absorbs the
// original prompt publish that fans in asynchronously after a request
// becomes pending: the fan-in goroutines subscribe to the service
// brokers in the background, so under load the original publish may
// arrive late or be dropped entirely. Either way, after a quiet
// window only a re-delivery triggered by SetCurrentSession can
// arrive. Re-deliveries publish straight into the workspace broker,
// which the tests subscribe to synchronously, so they race nothing.
func drainQuiet(events <-chan pubsub.Event[tea.Msg], window time.Duration) {
	timer := time.NewTimer(window)
	defer timer.Stop()
	for {
		select {
		case <-events:
		case <-timer.C:
			return
		}
	}
}

func TestClientCurrentSession(t *testing.T) {
	t.Parallel()
	b, ws, _ := newPromptsTestBackend(t)

	current, reported := b.ClientCurrentSession("no-such-workspace", "cid")
	require.Empty(t, current, "unknown workspace")
	require.False(t, reported, "unknown workspace")

	cid := uuid.New().String()
	current, reported = b.ClientCurrentSession(ws.ID, cid)
	require.Empty(t, current, "unknown client")
	require.False(t, reported, "unknown client")

	require.NoError(t, b.AttachClient(ws.ID, cid))
	current, reported = b.ClientCurrentSession(ws.ID, cid)
	require.Empty(t, current, "no session reported yet")
	require.False(t, reported, "no session reported yet")

	require.NoError(t, b.SetCurrentSession(t.Context(), ws.ID, cid, "S1"))
	current, reported = b.ClientCurrentSession(ws.ID, cid)
	require.Equal(t, "S1", current)
	require.True(t, reported)

	require.NoError(t, b.SetCurrentSession(t.Context(), ws.ID, cid, ""))
	current, reported = b.ClientCurrentSession(ws.ID, cid)
	require.Empty(t, current, "cleared session")
	require.True(t, reported, "clearing the selection is still a report")
}

func TestRootSessionID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := New(ctx, nil, nil)

	t.Run("workspace without app treats sessions as top-level", func(t *testing.T) {
		t.Parallel()
		ws := &Workspace{ID: uuid.New().String(), Path: t.TempDir()}
		InsertWorkspaceForTest(b, ws)

		root, err := b.RootSessionID(ctx, ws.ID, "anything")
		require.NoError(t, err)
		require.Equal(t, "anything", root)
	})

	t.Run("unknown workspace returns input and error", func(t *testing.T) {
		t.Parallel()
		root, err := b.RootSessionID(ctx, "no-such-workspace", "S1")
		require.Error(t, err)
		require.Equal(t, "S1", root)
	})

	t.Run("resolves the parent chain", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()
		conn, err := db.Connect(ctx, dataDir)
		require.NoError(t, err)
		t.Cleanup(func() { conn.Close() })
		sessions := session.NewService(db.New(conn), conn)

		ws := &Workspace{ID: uuid.New().String(), Path: t.TempDir(), App: &app.App{Sessions: sessions}}
		InsertWorkspaceForTest(b, ws)

		top, err := sessions.Create(ctx, "top")
		require.NoError(t, err)
		child, err := sessions.CreateTaskSession(ctx, "child-1", top.ID, "child")
		require.NoError(t, err)
		grandchild, err := sessions.CreateTaskSession(ctx, "grand-1", child.ID, "grandchild")
		require.NoError(t, err)

		root, err := b.RootSessionID(ctx, ws.ID, top.ID)
		require.NoError(t, err)
		require.Equal(t, top.ID, root, "top-level session resolves to itself")

		root, err = b.RootSessionID(ctx, ws.ID, child.ID)
		require.NoError(t, err)
		require.Equal(t, top.ID, root, "child resolves to parent")

		root, err = b.RootSessionID(ctx, ws.ID, grandchild.ID)
		require.NoError(t, err)
		require.Equal(t, top.ID, root, "grandchild resolves to the top-level session")

		root, err = b.RootSessionID(ctx, ws.ID, "ghost")
		require.Error(t, err, "unknown session must surface the lookup error")
		require.Equal(t, "ghost", root, "input is returned alongside the error")

		_, err = b.RootSessionID(ctx, ws.ID, "ghost")
		require.Error(t, err, "failed lookups must not be cached")
	})

	t.Run("memoizes successful resolutions", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()
		conn, err := db.Connect(ctx, dataDir)
		require.NoError(t, err)
		t.Cleanup(func() { conn.Close() })
		sessions := session.NewService(db.New(conn), conn)

		ws := &Workspace{ID: uuid.New().String(), Path: t.TempDir(), App: &app.App{Sessions: sessions}}
		InsertWorkspaceForTest(b, ws)

		top, err := sessions.Create(ctx, "top")
		require.NoError(t, err)
		child, err := sessions.CreateTaskSession(ctx, "child-1", top.ID, "child")
		require.NoError(t, err)

		root, err := b.RootSessionID(ctx, ws.ID, child.ID)
		require.NoError(t, err)
		require.Equal(t, top.ID, root)

		// With the child row gone, a second resolution would fail if
		// it hit the DB; the memoized answer is served instead.
		require.NoError(t, sessions.Delete(ctx, child.ID))
		root, err = b.RootSessionID(ctx, ws.ID, child.ID)
		require.NoError(t, err)
		require.Equal(t, top.ID, root)
	})
}

// TestSetCurrentSession_RepublishesPendingPermission verifies that a
// client switching into a session with a pending permission request
// re-receives the request on the workspace event stream, while
// switching to an unrelated session delivers nothing.
func TestSetCurrentSession_RepublishesPendingPermission(t *testing.T) {
	t.Parallel()
	b, ws, a := newPromptsTestBackend(t)
	ctx := context.Background()

	events, err := b.SubscribeEvents(ctx, ws.ID)
	require.NoError(t, err)

	reqCtx, cancelReq := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = a.Permissions.Request(reqCtx, permission.CreatePermissionRequest{
			SessionID:  "s-1",
			ToolCallID: "tc-1",
			ToolName:   "bash",
			Action:     "execute",
			Path:       ws.Path,
		})
	}()
	defer func() { cancelReq(); <-done }()

	var active permission.PermissionRequest
	require.Eventually(t, func() bool {
		pending := a.Permissions.ActiveRequests()
		if len(pending) == 0 {
			return false
		}
		active = pending[0]
		return true
	}, 10*time.Second, 5*time.Millisecond, "request must be pending")
	require.Equal(t, "tc-1", active.ToolCallID)

	// Absorb the original publish, then attach a client.
	drainQuiet(events, 300*time.Millisecond)

	cid := uuid.New().String()
	require.NoError(t, b.AttachClient(ws.ID, cid))

	// Switching to an unrelated session must not re-publish.
	require.NoError(t, b.SetCurrentSession(ctx, ws.ID, cid, "s-2"))
	select {
	case ev := <-events:
		t.Fatalf("unexpected event for unrelated session: %v", ev)
	case <-time.After(200 * time.Millisecond):
	}

	// Switching to the owning session must re-publish the request.
	require.NoError(t, b.SetCurrentSession(ctx, ws.ID, cid, "s-1"))
	select {
	case ev := <-events:
		reqEv, ok := ev.Payload.(pubsub.Event[permission.PermissionRequest])
		require.True(t, ok, "expected permission request event, got %T", ev.Payload)
		require.Equal(t, "s-1", reqEv.Payload.SessionID)
		require.Equal(t, "tc-1", reqEv.Payload.ToolCallID)
		require.Equal(t, active.ID, reqEv.Payload.ID, "same pending request")
	case <-time.After(10 * time.Second):
		t.Fatal("expected re-published permission request")
	}
}

// TestSetCurrentSession_RepublishesPendingQuestion verifies the same
// re-delivery for a pending question batch.
func TestSetCurrentSession_RepublishesPendingQuestion(t *testing.T) {
	t.Parallel()
	b, ws, a := newPromptsTestBackend(t)
	ctx := context.Background()

	events, err := b.SubscribeEvents(ctx, ws.ID)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = a.Questions.Ask(ctx, question.Request{
			SessionID: "s-1",
			Questions: []question.Question{{
				Type:        question.TypeFreeText,
				Text:        "Proceed?",
				Description: "Tell me what you think",
			}},
		})
	}()
	defer func() { a.Questions.Cancel(); <-done }()

	require.Eventually(t, func() bool {
		_, ok := a.Questions.Pending()
		return ok
	}, 10*time.Second, 5*time.Millisecond, "question must be pending")

	// Absorb the original publish, then attach a client.
	drainQuiet(events, 300*time.Millisecond)

	cid := uuid.New().String()
	require.NoError(t, b.AttachClient(ws.ID, cid))
	require.NoError(t, b.SetCurrentSession(ctx, ws.ID, cid, "s-1"))

	select {
	case ev := <-events:
		reqEv, ok := ev.Payload.(pubsub.Event[question.Request])
		require.True(t, ok, "expected question request event, got %T", ev.Payload)
		require.Equal(t, "s-1", reqEv.Payload.SessionID)
	case <-time.After(10 * time.Second):
		t.Fatal("expected re-published question request")
	}
}

// TestCreateWorkspace_PermissionPromptsWaitPerTopLevelSession verifies
// that a workspace's permission requests only wait for earlier requests
// of the same top-level session: another session's prompt is shown
// while one is left unanswered, and a sub-agent's prompt still waits
// for its parent session's, since the same clients answer both.
//
// Cannot run in parallel: it isolates HOME/XDG_* via t.Setenv so
// config.Init does not read the host machine's real config.
func TestCreateWorkspace_PermissionPromptsWaitPerTopLevelSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	ws, _, err := b.CreateWorkspace(protoWS(t.TempDir(), t.TempDir(), uuid.New().String()))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	// Unblock the requests on failure paths before the workspace goes.
	t.Cleanup(func() { cancel(); wg.Wait() })

	top, err := ws.Sessions.Create(ctx, "top")
	require.NoError(t, err)
	child, err := ws.Sessions.CreateTaskSession(ctx, "child-1", top.ID, "child")
	require.NoError(t, err)
	other, err := ws.Sessions.Create(ctx, "other")
	require.NoError(t, err)

	requests := ws.Permissions.Subscribe(ctx)
	request := func(sessionID string) <-chan bool {
		granted := make(chan bool, 1)
		wg.Go(func() {
			ok, _ := ws.Permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:  sessionID,
				ToolCallID: "tc-" + sessionID,
				ToolName:   "bash",
				Action:     "execute",
				Path:       ws.Path,
			})
			granted <- ok
		})
		return granted
	}
	next := func() permission.PermissionRequest {
		t.Helper()
		select {
		case ev := <-requests:
			return ev.Payload
		case <-time.After(10 * time.Second):
			t.Fatal("permission request was never published")
			return permission.PermissionRequest{}
		}
	}

	doneTop := request(top.ID)
	reqTop := next()
	require.Equal(t, top.ID, reqTop.SessionID)

	const childMustWait = "a sub-agent's prompt must wait for its top-level session's"
	doneChild := request(child.ID)
	doneOther := request(other.ID)
	reqOther := next()
	require.NotEqual(t, child.ID, reqOther.SessionID, childMustWait)
	require.Equal(t, other.ID, reqOther.SessionID,
		"another top-level session's prompt must not wait for an unanswered one")
	select {
	case ev := <-requests:
		t.Fatalf("%s, got %q", childMustWait, ev.Payload.SessionID)
	case <-time.After(200 * time.Millisecond):
	}

	require.True(t, ws.Permissions.Grant(reqOther))
	require.True(t, <-doneOther)
	require.True(t, ws.Permissions.Grant(reqTop))
	require.True(t, <-doneTop)
	reqChild := next()
	require.Equal(t, child.ID, reqChild.SessionID)
	require.True(t, ws.Permissions.Grant(reqChild))
	require.True(t, <-doneChild)
}
