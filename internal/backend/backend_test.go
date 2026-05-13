package backend

import (
	"context"
	"testing"

	"github.com/taigrr/crush/internal/config"
	"github.com/taigrr/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestCreateWorkspace_DeduplicatesByDataDir(t *testing.T) {
	tempDir := t.TempDir()

	cfg, err := config.Init(tempDir, "", false)
	require.NoError(t, err)

	b := New(context.Background(), cfg, nil)

	// Create first workspace.
	ws1, result1, err := b.CreateWorkspace(proto.Workspace{
		Path: tempDir,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result1.ID)

	// Create second workspace for the same directory.
	ws2, result2, err := b.CreateWorkspace(proto.Workspace{
		Path: tempDir,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result2.ID)

	// Should get different client IDs.
	require.NotEqual(t, result1.ID, result2.ID, "clients should have different IDs")

	// But should share the same workspace.
	require.Same(t, ws1, ws2, "should return the same workspace instance")

	// Both client IDs should resolve to the same workspace.
	got1, err := b.GetWorkspace(result1.ID)
	require.NoError(t, err)
	got2, err := b.GetWorkspace(result2.ID)
	require.NoError(t, err)
	require.Same(t, got1, got2, "both client IDs should resolve to same workspace")

	// Verify client tracking via the tracker's client count.
	wsID, ok := b.clients.workspaceForClient(result1.ID)
	require.True(t, ok)
	require.Equal(t, 2, b.clients.clientCount(wsID), "should have 2 clients tracked")
}

func TestDeleteWorkspace_OnlyDeletesWhenLastClientDisconnects(t *testing.T) {
	tempDir := t.TempDir()

	cfg, err := config.Init(tempDir, "", false)
	require.NoError(t, err)

	shutdownCalled := false
	b := New(context.Background(), cfg, func() {
		shutdownCalled = true
	})

	// Create two clients for the same directory.
	_, result1, err := b.CreateWorkspace(proto.Workspace{Path: tempDir})
	require.NoError(t, err)

	_, result2, err := b.CreateWorkspace(proto.Workspace{Path: tempDir})
	require.NoError(t, err)

	// Delete first client.
	b.DeleteWorkspace(result1.ID)

	// Workspace should still exist.
	_, err = b.GetWorkspace(result2.ID)
	require.NoError(t, err, "workspace should still exist after first client disconnects")
	require.False(t, shutdownCalled, "shutdown should not be called yet")

	// Delete second client.
	b.DeleteWorkspace(result2.ID)

	// Now workspace should be gone.
	_, err = b.GetWorkspace(result2.ID)
	require.Error(t, err, "workspace should be deleted after last client disconnects")
	require.True(t, shutdownCalled, "shutdown should be called when last workspace removed")
}

func TestDeleteWorkspace_CleansUpDataDirMapping(t *testing.T) {
	tempDir := t.TempDir()

	cfg, err := config.Init(tempDir, "", false)
	require.NoError(t, err)

	b := New(context.Background(), cfg, nil)

	// Create workspace.
	_, result1, err := b.CreateWorkspace(proto.Workspace{Path: tempDir})
	require.NoError(t, err)

	dataDir := result1.DataDir

	// Verify mapping exists.
	_, exists := b.clients.workspaceForDataDir(dataDir)
	require.True(t, exists, "data dir mapping should exist")

	// Delete workspace.
	b.DeleteWorkspace(result1.ID)

	// Verify mapping is cleaned up.
	_, exists = b.clients.workspaceForDataDir(dataDir)
	require.False(t, exists, "data dir mapping should be cleaned up")

	// Creating a new workspace should work.
	_, result2, err := b.CreateWorkspace(proto.Workspace{Path: tempDir})
	require.NoError(t, err)
	require.NotEqual(t, result1.ID, result2.ID, "new workspace should have new ID")
}
