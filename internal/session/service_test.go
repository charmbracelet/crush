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

func TestCreateUsesDefaultCollaborationModeAndAutoPermissionModeByDefault(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	svc := NewService(q, conn)

	created, err := svc.Create(context.Background(), "Auto by default")
	require.NoError(t, err)
	require.Equal(t, CollaborationModeDefault, created.CollaborationMode)
	require.Equal(t, PermissionModeAuto, created.PermissionMode)
}

func TestCreateCanUseExplicitDefaultMode(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	svc := NewService(q, conn, CollaborationModeDefault)

	created, err := svc.Create(context.Background(), "Manual mode")
	require.NoError(t, err)
	require.Equal(t, CollaborationModeDefault, created.CollaborationMode)
	require.Equal(t, PermissionModeAuto, created.PermissionMode)
}

func TestSetDefaultPermissionModeAffectsNewSessions(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	svc := NewService(q, conn)

	svc.SetDefaultPermissionMode(PermissionModeYolo)

	created, err := svc.Create(context.Background(), "Yolo default")
	require.NoError(t, err)
	require.Equal(t, PermissionModeYolo, created.PermissionMode)

	// Invalid mode inputs are normalized to default.
	svc.SetDefaultPermissionMode(PermissionMode("not-a-real-mode"))

	created, err = svc.Create(context.Background(), "Normalized default")
	require.NoError(t, err)
	require.Equal(t, PermissionModeDefault, created.PermissionMode)
}

func TestUpdatePermissionModePersistsAndNormalizes(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	svc := NewService(q, conn)

	created, err := svc.Create(context.Background(), "Permission mode updates")
	require.NoError(t, err)
	require.Equal(t, PermissionModeAuto, created.PermissionMode)

	updated, err := svc.UpdatePermissionMode(context.Background(), created.ID, PermissionModeYolo)
	require.NoError(t, err)
	require.Equal(t, PermissionModeYolo, updated.PermissionMode)

	loaded, err := svc.Get(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, PermissionModeYolo, loaded.PermissionMode)

	updated, err = svc.UpdatePermissionMode(context.Background(), created.ID, PermissionMode("invalid"))
	require.NoError(t, err)
	require.Equal(t, PermissionModeDefault, updated.PermissionMode)

	loaded, err = svc.Get(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, PermissionModeDefault, loaded.PermissionMode)
}

func TestUpdateModesNoopWhenStateUnchanged(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	svc := NewService(q, conn)

	created, err := svc.Create(context.Background(), "Noop mode updates")
	require.NoError(t, err)

	updated, err := svc.UpdatePermissionMode(context.Background(), created.ID, PermissionModeAuto)
	require.NoError(t, err)
	require.Equal(t, created.PermissionMode, updated.PermissionMode)

	updated, err = svc.UpdateCollaborationMode(context.Background(), created.ID, CollaborationModeDefault)
	require.NoError(t, err)
	require.Equal(t, created.CollaborationMode, updated.CollaborationMode)
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

func TestCreateTaskSessionInheritsParentModes(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	svc := NewService(q, conn)

	parent, err := svc.Create(context.Background(), "Parent")
	require.NoError(t, err)
	_, err = svc.UpdateCollaborationMode(context.Background(), parent.ID, CollaborationModePlan)
	require.NoError(t, err)
	_, err = svc.UpdatePermissionMode(context.Background(), parent.ID, PermissionModeYolo)
	require.NoError(t, err)

	child, err := svc.CreateTaskSession(context.Background(), "tool-1", parent.ID, "Child")
	require.NoError(t, err)
	require.Equal(t, parent.ID, child.ParentSessionID)
	require.Equal(t, CollaborationModePlan, child.CollaborationMode)
	require.Equal(t, PermissionModeYolo, child.PermissionMode)
}

func TestDeleteTriggersDeleteCallbackAfterSuccessfulDelete(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	deletedSessionIDs := make([]string, 0, 1)
	svc := NewServiceWithDeleteCallback(q, conn, func(sessionID string) {
		deletedSessionIDs = append(deletedSessionIDs, sessionID)
	})

	created, err := svc.Create(context.Background(), "Delete callback")
	require.NoError(t, err)

	err = svc.Delete(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, []string{created.ID}, deletedSessionIDs)
}

func TestDeleteDoesNotTriggerDeleteCallbackWhenDeleteFails(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	called := false
	svc := NewServiceWithDeleteCallback(q, conn, func(string) {
		called = true
	})

	err = svc.Delete(context.Background(), "missing-session")
	require.Error(t, err)
	require.False(t, called)
}
