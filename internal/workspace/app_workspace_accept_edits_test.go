package workspace

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// TestAppWorkspaceDeleteSessionClearsAcceptEdits: in-process mode drops a
// deleted session's accept-edits flag too; other sessions keep theirs.
func TestAppWorkspaceDeleteSessionClearsAcceptEdits(t *testing.T) {
	t.Parallel()
	a := app.NewForTest(context.Background())
	t.Cleanup(a.ShutdownForTest)
	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	a.Sessions = session.NewService(db.New(conn), conn)
	w := NewAppWorkspace(a, nil)

	gone, err := a.Sessions.Create(t.Context(), "gone")
	require.NoError(t, err)
	kept, err := a.Sessions.Create(t.Context(), "kept")
	require.NoError(t, err)
	w.PermissionSetAcceptEdits(gone.ID, true)
	w.PermissionSetAcceptEdits(kept.ID, true)

	require.NoError(t, w.DeleteSession(t.Context(), gone.ID))
	require.False(t, w.PermissionAcceptEdits(gone.ID))
	require.True(t, w.PermissionAcceptEdits(kept.ID))
}
