package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/projects"
	"github.com/stretchr/testify/require"
)

// isolateProjectList isolates config loading from the host as xdgIsolated
// does, keeps provider loading off the network, and pins CRUSH_GLOBAL_DATA,
// which takes precedence over XDG_DATA_HOME, so that registering a project
// never writes to the host's real projects.json. It returns the directory
// that holds projects.json.
func isolateProjectList(t *testing.T) string {
	t.Helper()
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	dir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	return dir
}

// TestCreateWorkspace_RegistersProject covers a project opened through the
// server. Local mode records every project it opens in projects.json, which
// is what `crush projects` and `crush stats --all` read, so the server has
// to record the projects it opens too.
func TestCreateWorkspace_RegistersProject(t *testing.T) {
	isolateProjectList(t)
	cwd := t.TempDir()
	dataDir := filepath.Join(cwd, ".crush")

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	_, _, err := b.CreateWorkspace(protoWS(cwd, dataDir, newClientID(t)))
	require.NoError(t, err)

	list, err := projects.List()
	require.NoError(t, err)
	require.Len(t, list, 1, "the server must register the project it opened")
	require.Equal(t, cwd, list[0].Path)
	require.Equal(t, dataDir, list[0].DataDir)
}

// TestCreateWorkspace_ProjectRegistrationIsBestEffort checks that
// registering the project is best effort, as it is in local mode: a
// projects.json that cannot be parsed must not keep the workspace from
// opening.
func TestCreateWorkspace_ProjectRegistrationIsBestEffort(t *testing.T) {
	dir := isolateProjectList(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projects.json"), []byte("{"), 0o600))
	cwd := t.TempDir()

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	_, _, err := b.CreateWorkspace(protoWS(cwd, filepath.Join(cwd, ".crush"), newClientID(t)))
	require.NoError(t, err)
}
