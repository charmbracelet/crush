package session

import (
	"database/sql"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestDeleteRemovesChildSessions(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	q := db.New(conn)
	svc := NewService(q, conn)

	parent, err := svc.Create(t.Context(), "parent")
	require.NoError(t, err)
	child, err := svc.CreateTaskSession(t.Context(), "msg$$call1", parent.ID, "child")
	require.NoError(t, err)
	grandchild, err := svc.CreateTaskSession(t.Context(), "msg$$call2", child.ID, "grandchild")
	require.NoError(t, err)

	other, err := svc.Create(t.Context(), "other")
	require.NoError(t, err)
	otherChild, err := svc.CreateTaskSession(t.Context(), "msg$$call3", other.ID, "other child")
	require.NoError(t, err)

	for _, sessionID := range []string{child.ID, grandchild.ID, otherChild.ID} {
		_, err = q.CreateFile(t.Context(), db.CreateFileParams{
			ID:        sessionID + "-file",
			SessionID: sessionID,
			Path:      "/a.go",
			Content:   "content",
		})
		require.NoError(t, err)
	}

	child.EstimatedUsage = true
	_, err = svc.Save(t.Context(), child)
	require.NoError(t, err)

	events := svc.Subscribe(t.Context())

	require.NoError(t, svc.Delete(t.Context(), parent.ID))

	for _, sessionID := range []string{parent.ID, child.ID, grandchild.ID} {
		_, err = q.GetSessionByID(t.Context(), sessionID)
		require.ErrorIs(t, err, sql.ErrNoRows, "session %s must be deleted", sessionID)
		files, err := q.ListFilesBySession(t.Context(), sessionID)
		require.NoError(t, err)
		require.Empty(t, files, "files of session %s must be deleted", sessionID)
	}

	impl := svc.(*service)
	impl.estimatedUsageMu.RLock()
	_, tracked := impl.estimatedUsage[child.ID]
	impl.estimatedUsageMu.RUnlock()
	require.False(t, tracked, "estimated usage of deleted children must be cleared")

	// Only the requested session is announced.
	ev := <-events
	require.Equal(t, pubsub.DeletedEvent, ev.Type)
	require.Equal(t, parent.ID, ev.Payload.ID)
	select {
	case ev := <-events:
		t.Fatalf("unexpected event %v for session %s", ev.Type, ev.Payload.ID)
	default:
	}

	// Unrelated sessions and their children are untouched.
	for _, sessionID := range []string{other.ID, otherChild.ID} {
		_, err = q.GetSessionByID(t.Context(), sessionID)
		require.NoError(t, err)
	}
	files, err := q.ListFilesBySession(t.Context(), otherChild.ID)
	require.NoError(t, err)
	require.Len(t, files, 1)
}
