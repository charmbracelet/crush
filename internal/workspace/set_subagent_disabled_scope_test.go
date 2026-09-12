package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/require"
)

// isolateConfigHome points config.Init's filesystem reads at a temp HOME so the
// test never sees (or writes) the developer's real config.
func isolateConfigHome(t *testing.T) {
	t.Helper()
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(hostHome, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(hostHome, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(hostHome, ".cache"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())
	t.Setenv("CRUSH_SUBAGENTS_DIR", t.TempDir())
}

// workspaceDisabledSubagents reads options.disabled_subagents straight out of
// the workspace config file, bypassing the merged in-memory view.
func workspaceDisabledSubagents(t *testing.T, store *config.ConfigStore) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store.Config().Options.DataDirectory, "crush.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		require.NoError(t, err)
	}
	var parsed struct {
		Options struct {
			DisabledSubagents []string `json:"disabled_subagents"`
		} `json:"options"`
	}
	require.NoError(t, json.Unmarshal(data, &parsed))
	return parsed.Options.DisabledSubagents
}

// TestSetSubagentDisabled_DoesNotCopyOtherScopes is the regression test for the
// workspace-scope write pulling in entries the user disabled globally.
// SetSubagentDisabled writes to ScopeWorkspace, so it must read from
// ScopeWorkspace too — reading the merged Config() would copy "from-global"
// into the workspace file, pinning it there even after the user removes it
// from their global config.
func TestSetSubagentDisabled_DoesNotCopyOtherScopes(t *testing.T) {
	isolateConfigHome(t)

	workDir := t.TempDir()
	store, err := config.Init(workDir, "", false)
	require.NoError(t, err)

	// Stand in for an entry inherited from the global scope: it is present in
	// the merged view but absent from the workspace config file.
	if store.Config().Options == nil {
		store.Config().Options = &config.Options{}
	}
	store.Config().Options.DisabledSubagents = []string{"from-global"}

	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: store,
	}

	require.NoError(t, w.SetSubagentDisabled("from-workspace", true))

	got := workspaceDisabledSubagents(t, store)
	require.Equal(t, []string{"from-workspace"}, got,
		"only the workspace-scope toggle may be written; the inherited entry must stay in its own scope")
}

// TestSetSubagentDisabled_RoundTripsWithinScope verifies the normal
// enable/disable cycle still works against the scoped read.
func TestSetSubagentDisabled_RoundTripsWithinScope(t *testing.T) {
	isolateConfigHome(t)

	workDir := t.TempDir()
	store, err := config.Init(workDir, "", false)
	require.NoError(t, err)

	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: store,
	}

	require.NoError(t, w.SetSubagentDisabled("alpha", true))
	require.NoError(t, w.SetSubagentDisabled("beta", true))
	require.ElementsMatch(t, []string{"alpha", "beta"}, workspaceDisabledSubagents(t, store))

	require.NoError(t, w.SetSubagentDisabled("alpha", false))
	require.Equal(t, []string{"beta"}, workspaceDisabledSubagents(t, store))
}
