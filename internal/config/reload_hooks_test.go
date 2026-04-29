package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

// TestReloadFromDisk_CompilesHookMatchers is a regression test for a bug
// where ReloadFromDisk dropped the compiled matcher regex on every hook,
// causing a matcher like "^bash$" to match every tool call after any
// SetConfigField-triggered reload.
//
// The assertion is phrased in terms of observable Runner behavior (not
// internal field presence) so it stays valid if the Runner later owns
// matcher compilation itself.
func TestReloadFromDisk_CompilesHookMatchers(t *testing.T) {
	// No t.Parallel(): we Setenv HOME/XDG_CONFIG_HOME to isolate from the
	// developer's real global config, which may define its own hooks.
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	configPath := filepath.Join(workDir, "crush.json")
	cfgJSON := `{
        "hooks": {
            "PreToolUse": [
                {"matcher": "^bash$", "command": "exit 0"}
            ]
        }
    }`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgJSON), 0o600))

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)

	// Sanity: hook filtering works immediately after Load.
	assertHookFilters(t, store)

	require.NoError(t, store.ReloadFromDisk(context.Background()))

	// The actual regression check: filtering must still work after a
	// reload, not silently collapse to match-everything.
	assertHookFilters(t, store)
}

// assertHookFilters builds a Runner from the store's current hooks and
// verifies the "^bash$" matcher rejects a non-bash tool while accepting
// bash.
func assertHookFilters(t *testing.T, store *config.ConfigStore) {
	t.Helper()
	preHooks := store.Config().Hooks[hooks.EventPreToolUse]
	require.Len(t, preHooks, 1)

	runner := hooks.NewRunner(preHooks, t.TempDir(), t.TempDir())

	nonMatch, err := runner.Run(context.Background(), hooks.EventPreToolUse, "sess", "view", `{}`)
	require.NoError(t, err)
	require.Equal(t, 0, nonMatch.HookCount, "view must not match ^bash$ matcher")

	match, err := runner.Run(context.Background(), hooks.EventPreToolUse, "sess", "bash", `{}`)
	require.NoError(t, err)
	require.Equal(t, 1, match.HookCount, "bash must match ^bash$ matcher")
}

// TestSetConfigField_AutoReload_PreservesHookMatcherFiltering verifies the
// dominant real-world trigger path: config writes call autoReload,
// autoReload calls ReloadFromDisk, and hook matching must remain correct.
func TestSetConfigField_AutoReload_PreservesHookMatcherFiltering(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	configPath := filepath.Join(workDir, "crush.json")
	cfgJSON := `{
        "hooks": {
            "PreToolUse": [
                {"matcher": "^bash$", "command": "exit 0"}
            ]
        }
    }`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgJSON), 0o600))

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	assertHookFilters(t, store)

	require.NoError(t, store.SetConfigField(config.ScopeGlobal, "options.debug", true))

	assertHookFilters(t, store)
}
