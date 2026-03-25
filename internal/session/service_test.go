package session

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func TestCreateHandoffSessionStoresMetadata(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	svc := NewService(q, conn)

	source, err := svc.Create(context.Background(), "Source")
	require.NoError(t, err)

	handoff, err := svc.CreateHandoffSession(
		context.Background(),
		source.ID,
		"Finish the refactor",
		"Continue the refactor in a fresh session",
		"Please continue the refactor and verify the tests.",
		[]string{"internal/app/app.go", "internal/ui/model/ui.go"},
	)
	require.NoError(t, err)
	require.Equal(t, KindHandoff, handoff.Kind)
	require.Empty(t, handoff.ParentSessionID)
	require.Equal(t, source.ID, handoff.HandoffSourceSessionID)
	require.Equal(t, "Continue the refactor in a fresh session", handoff.HandoffGoal)
	require.Equal(t, "Please continue the refactor and verify the tests.", handoff.HandoffDraftPrompt)
	require.Equal(t, []string{"internal/app/app.go", "internal/ui/model/ui.go"}, handoff.HandoffRelevantFiles)

	loaded, err := svc.Get(context.Background(), handoff.ID)
	require.NoError(t, err)
	require.Equal(t, handoff, loaded)
}

func TestSavePersistsHandoffDraftUpdates(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	svc := NewService(q, conn)

	source, err := svc.Create(context.Background(), "Source")
	require.NoError(t, err)
	handoff, err := svc.CreateHandoffSession(context.Background(), source.ID, "Title", "Goal", "Draft one", []string{"a.go"})
	require.NoError(t, err)

	handoff.HandoffDraftPrompt = "Draft two"
	handoff.HandoffRelevantFiles = []string{"a.go", "b.go"}
	saved, err := svc.Save(context.Background(), handoff)
	require.NoError(t, err)
	require.Equal(t, "Draft two", saved.HandoffDraftPrompt)
	require.Equal(t, []string{"a.go", "b.go"}, saved.HandoffRelevantFiles)

	loaded, err := svc.Get(context.Background(), handoff.ID)
	require.NoError(t, err)
	require.Equal(t, "Draft two", loaded.HandoffDraftPrompt)
	require.Equal(t, []string{"a.go", "b.go"}, loaded.HandoffRelevantFiles)
}

func TestListReturnsTopLevelHandoffSessions(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	svc := NewService(q, conn)

	source, err := svc.Create(context.Background(), "Source")
	require.NoError(t, err)
	_, err = svc.CreateHandoffSession(context.Background(), source.ID, "Handoff", "Goal", "Draft", []string{"a.go"})
	require.NoError(t, err)
	_, err = svc.CreateTaskSession(context.Background(), "tool-1", source.ID, "Child")
	require.NoError(t, err)

	list, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, list, 2)

	var handoffFound bool
	for _, sess := range list {
		require.Empty(t, sess.ParentSessionID)
		if sess.Kind == KindHandoff {
			handoffFound = true
			require.Equal(t, source.ID, sess.HandoffSourceSessionID)
		}
	}
	require.True(t, handoffFound)
}
