package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/asx8678/ultra/internal/config"
	"github.com/stretchr/testify/require"
)

// isolateReloadEnv points HOME/XDG/ULTRA_* at a throwaway directory so a
// reload only sees the config files the test writes. No t.Parallel(): these
// tests Setenv.
func isolateReloadEnv(t *testing.T) (workDir, dataDir string) {
	t.Helper()
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("ULTRA_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "ultra"))
	t.Setenv("ULTRA_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "ultra"))
	return t.TempDir(), t.TempDir()
}

// TestReloadFromDisk_PicksUpEditedUltrarc verifies that a ultrarc edit on
// disk is reflected in memory after a reload, matching how JSON config
// already behaves.
func TestReloadFromDisk_PicksUpEditedUltrarc(t *testing.T) {
	workDir, dataDir := isolateReloadEnv(t)
	rcPath := filepath.Join(workDir, "ultrarc")
	require.NoError(t, os.WriteFile(rcPath, []byte("option notifications bell\n"), 0o644))

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	require.Equal(t, "bell", store.Config().Options.Notifications)

	require.NoError(t, os.WriteFile(rcPath, []byte("option notifications osc\n"), 0o644))
	require.NoError(t, store.ReloadFromDisk(context.Background()))
	require.Equal(t, "osc", store.Config().Options.Notifications,
		"reload must re-execute the ultrarc and pick up the new value")
}

// TestReloadFromDisk_FailingUltrarcKeepsOldConfig verifies that a ultrarc
// which fails to load during a reload does not clobber the in-memory config.
// The reload must return an error and leave the previously-loaded config
// intact (the swap happens only after a successful parse).
func TestReloadFromDisk_FailingUltrarcKeepsOldConfig(t *testing.T) {
	workDir, dataDir := isolateReloadEnv(t)
	rcPath := filepath.Join(workDir, "ultrarc")
	require.NoError(t, os.WriteFile(rcPath, []byte("option notifications bell\n"), 0o644))

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	require.Equal(t, "bell", store.Config().Options.Notifications)

	// Overwrite with a ultrarc that errors at load time (unknown option key).
	require.NoError(t, os.WriteFile(rcPath, []byte("option totally-bogus-key value\n"), 0o644))

	err = store.ReloadFromDisk(context.Background())
	require.Error(t, err, "a failing ultrarc must surface a reload error")
	require.Equal(t, "bell", store.Config().Options.Notifications,
		"in-memory config must be preserved when a reload fails")
}

// TestReloadFromDisk_HangingUltrarcIsInterruptible is the core regression test
// for the reload-path blocker: config reloads run while holding the store's
// write lock, so a ultrarc that blocks (busy loop, hung command substitution)
// must be interruptible via the reload context rather than wedging the store
// forever. A cancelled reload must return an error and leave the old config
// in place. The test bounds its own wait so a regression can't hang CI.
func TestReloadFromDisk_HangingUltrarcIsInterruptible(t *testing.T) {
	workDir, dataDir := isolateReloadEnv(t)
	rcPath := filepath.Join(workDir, "ultrarc")
	require.NoError(t, os.WriteFile(rcPath, []byte("option notifications bell\n"), 0o644))

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	require.Equal(t, "bell", store.Config().Options.Notifications)

	// Overwrite with a ultrarc that never terminates.
	require.NoError(t, os.WriteFile(rcPath, []byte("while true; do :; done\n"), 0o644))

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() {
		done <- store.ReloadFromDisk(ctx)
	}()

	select {
	case err := <-done:
		require.Error(t, err, "a cancelled reload must fail, not succeed")
	case <-time.After(3 * time.Second):
		t.Fatal("ReloadFromDisk did not return after context cancellation; the store would be wedged")
	}

	require.Equal(t, "bell", store.Config().Options.Notifications,
		"in-memory config must be preserved when a reload is cancelled")
}

// TestLoad_TracksNotYetCreatedGlobalUltrarc verifies that config.Load tracks
// the global ultrarc path even when the file does not exist yet, so a ultrarc
// created after startup is detected as a staleness change. Previously only
// successfully-loaded paths were tracked, so a mid-session global ultrarc went
// unnoticed until something else triggered a reload.
//
// Project-level ultrarc files are discovered by walking the tree for existing
// files, so a not-yet-created project ultrarc is still only picked up on the
// next reload; the global path is the common case this covers.
func TestLoad_TracksNotYetCreatedGlobalUltrarc(t *testing.T) {
	workDir, dataDir := isolateReloadEnv(t)
	globalRC := filepath.Join(t.TempDir(), "ultrarc")
	t.Setenv("ULTRA_GLOBAL_CONFIG", filepath.Dir(globalRC))

	// A provider must be configured so Load runs past its early
	// "not configured" return and reaches the staleness snapshot capture.
	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "ultrarc"),
		[]byte("provider add openai --api-key k\n"), 0o644,
	))

	// Load with no global ultrarc present.
	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	require.False(t, store.ConfigStaleness().Dirty, "fresh load should be clean")

	// Create the global ultrarc after startup.
	require.NoError(t, os.WriteFile(globalRC, []byte("option debug true\n"), 0o644))

	staleness := store.ConfigStaleness()
	require.True(t, staleness.Dirty, "creating a global ultrarc must be detected")
	require.Contains(t, staleness.Changed, globalRC)
}
