package server

import (
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestSessionScopedEventID verifies that exactly the four interactive
// prompt event types are treated as session-scoped, and that their
// session IDs are extracted for the delivery filter.
func TestSessionScopedEventID(t *testing.T) {
	t.Parallel()

	scoped := []struct {
		name string
		ev   any
		id   string
	}{
		{"permission request", pubsub.Event[permission.PermissionRequest]{Payload: permission.PermissionRequest{SessionID: "s1"}}, "s1"},
		{"permission notification", pubsub.Event[permission.PermissionNotification]{Payload: permission.PermissionNotification{SessionID: "s2"}}, "s2"},
		{"question request", pubsub.Event[question.Request]{Payload: question.Request{SessionID: "s3"}}, "s3"},
		{"question notification", pubsub.Event[question.Notification]{Payload: question.Notification{SessionID: "s4"}}, "s4"},
	}
	for _, tc := range scoped {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id, ok := sessionScopedEventID(tc.ev)
			require.True(t, ok)
			require.Equal(t, tc.id, id)
		})
	}

	unscoped := []struct {
		name string
		ev   any
	}{
		{"session event", pubsub.Event[session.Session]{Payload: session.Session{ID: "s1"}}},
		{"config changed", pubsub.Event[struct{}]{}},
		{"string", "not an event"},
	}
	for _, tc := range unscoped {
		t.Run("unscoped/"+tc.name, func(t *testing.T) {
			t.Parallel()
			_, ok := sessionScopedEventID(tc.ev)
			require.False(t, ok)
		})
	}
}

// deliverFixture builds a controller with a synthetic workspace and
// two attached clients, one viewing session S1 and one viewing S2.
func deliverFixture(t *testing.T) (*controllerV1, *backend.Workspace, string, string) {
	t.Helper()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)

	cidA := uuid.New().String()
	cidB := uuid.New().String()
	require.NoError(t, c.backend.AttachClient(ws.ID, cidA))
	require.NoError(t, c.backend.AttachClient(ws.ID, cidB))
	t.Cleanup(func() {
		c.backend.DetachClient(ws.ID, cidA)
		c.backend.DetachClient(ws.ID, cidB)
	})
	require.NoError(t, c.backend.SetCurrentSession(t.Context(), ws.ID, cidA, "S1"))
	require.NoError(t, c.backend.SetCurrentSession(t.Context(), ws.ID, cidB, "S2"))
	return c, ws, cidA, cidB
}

// TestDeliverToClient covers the per-client SSE delivery filter:
// session-scoped prompt events only reach clients viewing the owning
// session; everything else passes through untouched.
func TestDeliverToClient(t *testing.T) {
	t.Parallel()

	permReq := pubsub.Event[permission.PermissionRequest]{Payload: permission.PermissionRequest{SessionID: "S1"}}
	permNotif := pubsub.Event[permission.PermissionNotification]{Payload: permission.PermissionNotification{SessionID: "S1", Granted: true}}
	qReq := pubsub.Event[question.Request]{Payload: question.Request{SessionID: "S1"}}
	qNotif := pubsub.Event[question.Notification]{Payload: question.Notification{SessionID: "S1"}}
	sessionEv := pubsub.Event[session.Session]{Payload: session.Session{ID: "S1"}}

	t.Run("viewer receives, non-viewer does not", func(t *testing.T) {
		t.Parallel()
		c, ws, cidA, cidB := deliverFixture(t)
		for _, ev := range []any{permReq, permNotif, qReq, qNotif} {
			require.True(t, c.deliverToClient(t.Context(), ws.ID, cidA, ev), "%T must reach the viewer", ev)
			require.False(t, c.deliverToClient(t.Context(), ws.ID, cidB, ev), "%T must not reach the non-viewer", ev)
		}
	})

	t.Run("two clients viewing the same session both receive", func(t *testing.T) {
		t.Parallel()
		c, ws, cidA, cidB := deliverFixture(t)
		require.NoError(t, c.backend.SetCurrentSession(t.Context(), ws.ID, cidB, "S1"))
		require.True(t, c.deliverToClient(t.Context(), ws.ID, cidA, permReq))
		require.True(t, c.deliverToClient(t.Context(), ws.ID, cidB, permReq))
	})

	t.Run("client viewing no session receives nothing scoped", func(t *testing.T) {
		t.Parallel()
		c, ws, cidA, _ := deliverFixture(t)
		require.NoError(t, c.backend.SetCurrentSession(t.Context(), ws.ID, cidA, ""))
		for _, ev := range []any{permReq, permNotif, qReq, qNotif} {
			require.False(t, c.deliverToClient(t.Context(), ws.ID, cidA, ev), "%T must not reach a session-less client", ev)
		}
	})

	t.Run("unscoped events always pass", func(t *testing.T) {
		t.Parallel()
		c, ws, cidA, cidB := deliverFixture(t)
		require.NoError(t, c.backend.SetCurrentSession(t.Context(), ws.ID, cidA, ""))
		require.True(t, c.deliverToClient(t.Context(), ws.ID, cidA, sessionEv))
		require.True(t, c.deliverToClient(t.Context(), ws.ID, cidB, sessionEv))
	})
}

// TestDeliverToClient_SubSessionResolvesToParent verifies that a
// prompt raised by a sub-agent session (a task session with a parent)
// is delivered to clients viewing the top-level parent session, and
// that an unresolvable session fails open rather than stranding the
// prompt.
func TestDeliverToClient_SubSessionResolvesToParent(t *testing.T) {
	t.Parallel()
	c, ws, cidA, cidB := deliverFixture(t)

	// Equip the workspace with a real session service so the parent
	// chain is resolvable.
	a := app.NewForTest(t.Context())
	t.Cleanup(a.ShutdownForTest)
	dataDir := t.TempDir()
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	a.Sessions = session.NewService(db.New(conn), conn)
	ws.App = a

	parent, err := a.Sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	child, err := a.Sessions.CreateTaskSession(t.Context(), "child-1", parent.ID, "child")
	require.NoError(t, err)

	require.NoError(t, c.backend.SetCurrentSession(t.Context(), ws.ID, cidA, parent.ID))

	childReq := pubsub.Event[permission.PermissionRequest]{Payload: permission.PermissionRequest{SessionID: child.ID}}
	require.True(t, c.deliverToClient(t.Context(), ws.ID, cidA, childReq),
		"sub-session prompt must reach the parent session's viewer")
	require.False(t, c.deliverToClient(t.Context(), ws.ID, cidB, childReq),
		"sub-session prompt must not reach another session's viewer")

	// A session that cannot be resolved (not in the DB) fails open:
	// better to broadcast than to strand a prompt nobody can answer.
	ghostReq := pubsub.Event[permission.PermissionRequest]{Payload: permission.PermissionRequest{SessionID: "ghost"}}
	require.True(t, c.deliverToClient(t.Context(), ws.ID, cidB, ghostReq))
}
