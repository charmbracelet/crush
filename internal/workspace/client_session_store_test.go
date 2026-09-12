package workspace_test

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// TestClientSessionStore_ThinClientSessionOps drives every
// ClientSessionStore operation through the real server handler: the
// session subcommands in client/server mode read and mutate sessions on
// the server, never the local database.
func TestClientSessionStore_ThinClientSessionOps(t *testing.T) {
	xdgIsolate(t)
	rt := newRuntimeServer(t)

	cwd := t.TempDir()
	dataDir := t.TempDir()

	c := rt.newClient(t, cwd)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ws, err := c.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir: dataDir})
	require.NoError(t, err)

	created, err := c.CreateSession(ctx, ws.ID, "original title")
	require.NoError(t, err)

	store := workspace.NewClientSessionStore(c, ws.ID)

	list, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, created.ID, list[0].ID)
	require.Equal(t, "original title", list[0].Title)

	got, err := store.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "original title", got.Title)

	require.NoError(t, store.Rename(ctx, created.ID, "renamed title"))
	got, err = store.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "renamed title", got.Title, "rename must round-trip through the server")

	msgs, err := store.ListMessages(ctx, created.ID)
	require.NoError(t, err)
	require.Empty(t, msgs)

	require.NoError(t, store.Delete(ctx, created.ID))
	list, err = store.List(ctx)
	require.NoError(t, err)
	require.Empty(t, list)
}
