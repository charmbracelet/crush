package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
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
	return workspaceSubagentOptions(t, store).DisabledSubagents
}

// workspaceSubagentOptions reads the subagent enable/disable lists straight
// out of the workspace config file.
func workspaceSubagentOptions(t *testing.T, store *config.ConfigStore) (opts struct {
	DisabledSubagents []string `json:"disabled_subagents"`
	EnabledSubagents  []string `json:"enabled_subagents"`
},
) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store.Config().Options.DataDirectory, "crush.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return opts
		}
		require.NoError(t, err)
	}
	var parsed struct {
		Options *struct {
			DisabledSubagents []string `json:"disabled_subagents"`
			EnabledSubagents  []string `json:"enabled_subagents"`
		} `json:"options"`
	}
	require.NoError(t, json.Unmarshal(data, &parsed))
	if parsed.Options != nil {
		opts.DisabledSubagents = parsed.Options.DisabledSubagents
		opts.EnabledSubagents = parsed.Options.EnabledSubagents
	}
	return opts
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
	// Only the workspace disabled alpha, so removing that entry is enough; a
	// lingering enabled_subagents entry would silently override any later
	// global disable.
	require.Empty(t, workspaceSubagentOptions(t, store).EnabledSubagents)
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

	// Disabled at workspace scope as well: enabling must still record the
	// override, since dropping the workspace entry alone leaves the global
	// one in effect.
	require.NoError(t, w.SetSubagentDisabled("global-agent", true))
	require.NoError(t, w.SetSubagentDisabled("global-agent", false))
	require.Equal(t, []string{"global-agent"}, workspaceSubagentOptions(t, store).EnabledSubagents)

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

// TestSetSubagentDisabled_ConcurrentTogglesKeepEveryWrite verifies that
// concurrent Library toggles, which run as separate commands, do not
// overwrite each other's read-modify-write of the workspace config.
func TestSetSubagentDisabled_ConcurrentTogglesKeepEveryWrite(t *testing.T) {
	isolateConfigHome(t)

	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)
	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)
	w := &AppWorkspace{app: &app.App{Subagents: mgr}, store: store}

	names := []string{"a", "b", "c", "d", "e", "f"}
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Go(func() {
			require.NoError(t, w.SetSubagentDisabled(name, true))
		})
	}
	wg.Wait()
	require.ElementsMatch(t, names, workspaceDisabledSubagents(t, store))
}

// TestSetSubagentDisabled_BroaderEnableRefusesDisable verifies that the
// Library reports an error instead of writing a disable that an
// enabled_subagents entry from another scope would silently override.
func TestSetSubagentDisabled_BroaderEnableRefusesDisable(t *testing.T) {
	isolateConfigHome(t)

	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)
	require.NoError(t, store.SetConfigField(config.ScopeGlobal, "options.enabled_subagents", []string{"reviewer"}))
	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)
	w := &AppWorkspace{app: &app.App{Subagents: mgr}, store: store}

	require.ErrorContains(t, w.SetSubagentDisabled("reviewer", true), "enabled_subagents")
	require.Empty(t, workspaceDisabledSubagents(t, store))
}

// newSubagentTestWorkspace builds a workspace over a real store with one
// global subagent definition per name, discovered into the manager.
func newSubagentTestWorkspace(t *testing.T, names ...string) (*AppWorkspace, string) {
	t.Helper()
	isolateConfigHome(t)
	dir := os.Getenv("CRUSH_SUBAGENTS_DIR")
	for _, n := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n+".md"),
			[]byte("---\nname: "+n+"\ndescription: d\n---\nbody\n"), 0o644))
	}
	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)
	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)
	w := &AppWorkspace{app: &app.App{Subagents: mgr}, store: store}
	w.reloadSubagents()
	require.Len(t, mgr.ActiveSubagents(), len(names))
	return w, dir
}

func activeNames(w *AppWorkspace) []string {
	var names []string
	for _, s := range w.app.Subagents.ActiveSubagents() {
		names = append(names, s.Name)
	}
	return names
}

// TestSubagentMutations_ConcurrentMatchDisk verifies concurrent toggles and
// deletes, racing other config writes that make autoReload skip, leave the
// manager's active set matching what is on disk.
func TestSubagentMutations_ConcurrentMatchDisk(t *testing.T) {
	for range 20 {
		w, _ := newSubagentTestWorkspace(t, "a", "b", "c", "d", "e", "f")

		stop := make(chan struct{})
		var hammer sync.WaitGroup
		hammer.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					// Holds the store's write lock, so a concurrent
					// write's autoReload skips.
					w.store.OverridePreferredModel(config.SelectedModelTypeLarge, config.SelectedModel{})
				}
			}
		})
		var wg sync.WaitGroup
		for _, n := range []string{"a", "b", "c"} {
			wg.Go(func() { require.NoError(t, w.SetSubagentDisabled(n, true)) })
		}
		wg.Go(func() { require.NoError(t, w.DeleteUserSubagent("d")) })
		wg.Wait()
		close(stop)
		hammer.Wait()

		require.ElementsMatch(t, []string{"e", "f"}, activeNames(w))
	}
}

// TestDeleteUserSubagent_AlreadyRemoved verifies deleting a definition that
// was removed outside Crush succeeds and clears it from the manager.
func TestDeleteUserSubagent_AlreadyRemoved(t *testing.T) {
	w, dir := newSubagentTestWorkspace(t, "gone", "kept")
	require.NoError(t, os.Remove(filepath.Join(dir, "gone.md")))

	require.NoError(t, w.DeleteUserSubagent("gone"))
	require.Equal(t, []string{"kept"}, activeNames(w))
}

// TestDeleteUserSubagent_ClearsConfigEntries verifies a deleted name does not
// linger in the workspace enable/disable lists, where it would silently
// apply to a later definition with the same name.
func TestDeleteUserSubagent_ClearsConfigEntries(t *testing.T) {
	w, _ := newSubagentTestWorkspace(t, "reviewer")
	require.NoError(t, w.SetSubagentDisabled("reviewer", true))
	require.NoError(t, w.store.SetConfigField(config.ScopeWorkspace, "options.enabled_subagents", []string{"reviewer"}))

	require.NoError(t, w.DeleteUserSubagent("reviewer"))
	opts := workspaceSubagentOptions(t, w.store)
	require.Empty(t, opts.DisabledSubagents)
	require.Empty(t, opts.EnabledSubagents)
}
