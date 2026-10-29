package backend

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/projects"
	"github.com/stretchr/testify/require"
)

// TestCreateWorkspace_RegistersProject guards against the server not
// recording the projects it opens in projects.json, as local mode does.
func TestCreateWorkspace_RegistersProject(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	cwd := t.TempDir()
	dataDir := filepath.Join(cwd, ".crush")

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })
	_, _, err := b.CreateWorkspace(protoWS(cwd, dataDir, newClientID(t)))
	require.NoError(t, err)

	list, err := projects.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, cwd, list[0].Path)
	require.Equal(t, dataDir, list[0].DataDir)
}
