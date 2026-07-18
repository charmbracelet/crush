package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// loadCrushSh writes a crush.sh into an isolated project and loads it through
// the real config pipeline (discovery -> shell execution -> merge -> typed
// Config). Asserting on the resulting *config.Config is a black-box test of
// what a shell config command actually produces, and it stays valid across
// internal changes to how config is assembled.
func loadCrushSh(t *testing.T, script string) *config.ConfigStore {
	t.Helper()
	store, err := loadCrushShErr(t, script)
	require.NoError(t, err)
	return store
}

// loadCrushShErr is loadCrushSh without asserting success, for cases that are
// expected to fail at load time.
func loadCrushShErr(t *testing.T, script string) (*config.ConfigStore, error) {
	t.Helper()
	// Isolate from the developer's real global config so only the script
	// under test contributes. No t.Parallel(): these tests set env vars.
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "crush.sh"), []byte(script), 0o644))

	return config.Load(workDir, dataDir, false)
}

func TestShellConfigPermissionsAllow(t *testing.T) {
	store := loadCrushSh(t, `permissions allow bash view`)

	require.NotNil(t, store.Config().Permissions)
	require.ElementsMatch(t, []string{"bash", "view"}, store.Config().Permissions.AllowedTools)
}

func TestShellConfigPermissionsAccumulateAndDedup(t *testing.T) {
	store := loadCrushSh(t, `permissions allow bash
permissions allow view
permissions allow bash`)

	require.Equal(t, []string{"bash", "view"}, store.Config().Permissions.AllowedTools)
}

func TestShellConfigPermissionsLegacyFlagFails(t *testing.T) {
	_, err := loadCrushShErr(t, `permissions --allow bash`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown subcommand")
}

func TestShellConfigPermissionsAllowRequiresTool(t *testing.T) {
	_, err := loadCrushShErr(t, `permissions allow`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage: permissions allow")
}
