package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/require"
)

// TestReloadSubagents_ExpandsEnvVarPath verifies that reloadSubagents
// passes the store's resolver through to subagent discovery, so a
// "$VAR"-style entry in Options.SubagentsPaths is expanded and the
// subagent it names is discovered. Before the fix, the "$VAR" entry is
// walked literally (no directory named "$CRUSH_TEST_SA_DIR" exists) and
// discovery finds nothing.
func TestReloadSubagents_ExpandsEnvVarPath(t *testing.T) {
	// Isolate config.Init's filesystem reads from the host, matching the
	// backend package's skills-discovery test setup.
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(hostHome, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(hostHome, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(hostHome, ".cache"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())
	t.Setenv("CRUSH_SUBAGENTS_DIR", t.TempDir())

	saDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(saDir, "env-agent.md"),
		[]byte("---\nname: env-agent\ndescription: Discovered through an env-var path.\n---\n\nYou are a test agent.\n"),
		0o644,
	))

	t.Setenv("CRUSH_TEST_SA_DIR", saDir)

	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)

	store.Config().Options.SubagentsPaths = append(
		store.Config().Options.SubagentsPaths, "$CRUSH_TEST_SA_DIR",
	)

	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: store,
	}

	w.reloadSubagents()

	found := false
	for _, a := range mgr.ActiveSubagents() {
		if a.Name == "env-agent" {
			found = true
			break
		}
	}
	require.True(t, found, "expected env-agent to be discovered via the $CRUSH_TEST_SA_DIR resolver-expanded path")
}
