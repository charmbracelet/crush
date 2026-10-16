package app

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/require"
)

// TestReloadSubagents_PicksUpNewProviderModel verifies that rediscovery
// re-validates model: against the current config, so a subagent hidden at
// startup because its model wasn't configured becomes active once a
// provider offering it is added, without a restart.
func TestReloadSubagents_PicksUpNewProviderModel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())
	saDir := t.TempDir()
	t.Setenv("CRUSH_SUBAGENTS_DIR", saDir)
	require.NoError(t, os.WriteFile(filepath.Join(saDir, "late.md"),
		[]byte("---\nname: late\ndescription: d.\nmodel: late-model\n---\n\nBody.\n"), 0o644))

	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)
	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)
	app := &App{Subagents: mgr, config: store}

	app.reloadSubagents()
	require.Empty(t, mgr.ActiveSubagents())

	store.Config().Providers.Set("late-provider", config.ProviderConfig{
		ID:     "late-provider",
		Models: []catwalk.Model{{ID: "late-model"}},
	})
	app.reloadSubagents()
	active := mgr.ActiveSubagents()
	require.Len(t, active, 1)
	require.Equal(t, "late", active[0].Name)
}
