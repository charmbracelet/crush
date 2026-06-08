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

// TestSetSubagentDisabled_EnableOverridesBroaderScope is the regression test
// for the actual user-facing bug: a name disabled at a broader (e.g. global)
// scope can never be re-enabled from workspace scope, because
// SetSubagentDisabled only ever subtracts from options.disabled_subagents at
// workspace scope — it has no way to cancel out an entry that lives in a
// different config layer, since jsons.Merge concatenates arrays across
// layers rather than overriding them. Enabling must leave AllSubagents (what
// the Library, dispatcher enum, and dispatch lookup all derive from) showing
// the name as no longer disabled, regardless of which scope disabled it.
func TestSetSubagentDisabled_EnableOverridesBroaderScope(t *testing.T) {
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(hostHome, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(hostHome, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(hostHome, ".cache"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())

	// A real on-disk definition, since SetSubagentDisabled's reload
	// re-discovers from disk and would otherwise wipe out a seeded fake
	// manager entry.
	saDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(saDir, "global-agent.md"),
		[]byte("---\nname: global-agent\ndescription: Lives in the global scope.\n---\n\nBody.\n"),
		0o644,
	))
	t.Setenv("CRUSH_SUBAGENTS_DIR", saDir)

	workDir := t.TempDir()
	store, err := config.Init(workDir, "", false)
	require.NoError(t, err)

	// Disable at global scope, persisted to disk so it survives the config
	// reload triggered by the workspace-scope write below — an in-memory-only
	// mutation would be silently discarded by that reload, masking the bug.
	require.NoError(t, store.SetConfigField(config.ScopeGlobal, "options.disabled_subagents", []string{"global-agent"}))

	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: store,
	}

	require.NoError(t, w.SetSubagentDisabled("global-agent", false))

	var found *SubagentDefInfo
	for _, info := range w.AllSubagents() {
		if info.Name == "global-agent" {
			cp := info
			found = &cp
			break
		}
	}
	require.NotNil(t, found, "global-agent should still be discovered")
	require.False(t, found.Disabled,
		"enabling at workspace scope must override an entry disabled at a broader scope")
}
