package backend

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// TestDeleteSessionClearsAcceptEdits: deleting a session drops its
// accept-edits flag; other sessions keep theirs.
func TestDeleteSessionClearsAcceptEdits(t *testing.T) {
	t.Parallel()
	b, ws, a := newPromptsTestBackend(t)
	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	a.Sessions = session.NewService(db.New(conn), conn)

	gone, err := a.Sessions.Create(t.Context(), "gone")
	require.NoError(t, err)
	kept, err := a.Sessions.Create(t.Context(), "kept")
	require.NoError(t, err)
	require.NoError(t, b.SetPermissionsAcceptEdits(ws.ID, gone.ID, true))
	require.NoError(t, b.SetPermissionsAcceptEdits(ws.ID, kept.ID, true))

	require.NoError(t, b.DeleteSession(t.Context(), ws.ID, gone.ID))
	on, err := b.GetPermissionsAcceptEdits(ws.ID, gone.ID)
	require.NoError(t, err)
	require.False(t, on)
	on, err = b.GetPermissionsAcceptEdits(ws.ID, kept.ID)
	require.NoError(t, err)
	require.True(t, on)
}
