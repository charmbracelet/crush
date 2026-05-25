package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func isolateProviderStateForTest(t *testing.T) {
	t.Helper()

	// Mutates package-level provider singletons; callers must not use t.Parallel().
	originalList := providerList
	originalErr := providerErr
	originalCatwalkSyncer := catwalkSyncer
	originalHyperSyncer := hyperSyncer

	providerOnce = sync.Once{}
	providerList = nil
	providerErr = nil
	catwalkSyncer = &catwalkSync{}
	hyperSyncer = &hyperSync{}

	t.Cleanup(func() {
		providerOnce = sync.Once{}
		providerList = originalList
		providerErr = originalErr
		catwalkSyncer = originalCatwalkSyncer
		hyperSyncer = originalHyperSyncer
	})
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func modelSelectionTestConfig(models map[string]any, recentModels map[string]any) map[string]any {
	cfg := map[string]any{
		"options": map[string]any{
			"disable_default_providers": true,
			"data_directory":            ".crush",
		},
		"providers": map[string]any{
			"test-provider": map[string]any{
				"api_key":  "test-key",
				"base_url": "https://example.com/v1",
				"models": []map[string]any{
					{"id": "global-large", "name": "Global Large", "default_max_tokens": 1000},
					{"id": "global-small", "name": "Global Small", "default_max_tokens": 500},
					{"id": "workspace-large", "name": "Workspace Large", "default_max_tokens": 2000},
					{"id": "workspace-small", "name": "Workspace Small", "default_max_tokens": 700},
				},
			},
		},
	}
	if models != nil {
		cfg["models"] = models
	}
	if recentModels != nil {
		cfg["recent_models"] = recentModels
	}
	return cfg
}

func TestConfigStore_ConfigPath_GlobalAlwaysWorks(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{
		globalDataPath: "/some/global/crush.json",
	}

	path, err := store.configPath(ScopeGlobal)
	require.NoError(t, err)
	require.Equal(t, "/some/global/crush.json", path)
}

func TestConfigStore_ConfigPath_WorkspaceReturnsPath(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{
		workspacePath: "/some/workspace/.crush/crush.json",
	}

	path, err := store.configPath(ScopeWorkspace)
	require.NoError(t, err)
	require.Equal(t, "/some/workspace/.crush/crush.json", path)
}

func TestConfigStore_ConfigPath_WorkspaceErrorsWhenEmpty(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{
		globalDataPath: "/some/global/crush.json",
		workspacePath:  "",
	}

	_, err := store.configPath(ScopeWorkspace)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoWorkspaceConfig))
}

func TestConfigStore_SetConfigField_WorkspaceScopeGuard(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: filepath.Join(t.TempDir(), "global.json"),
		workspacePath:  "",
	}

	err := store.SetConfigField(ScopeWorkspace, "foo", "bar")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoWorkspaceConfig))
}

func TestConfigStore_SetConfigField_GlobalScopeAlwaysWorks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	globalPath := filepath.Join(dir, "crush.json")
	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: globalPath,
	}

	err := store.SetConfigField(ScopeGlobal, "foo", "bar")
	require.NoError(t, err)

	data, err := os.ReadFile(globalPath)
	require.NoError(t, err)
	require.Contains(t, string(data), `"foo"`)
}

func TestConfigStore_RemoveConfigField_WorkspaceScopeGuard(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: filepath.Join(t.TempDir(), "global.json"),
		workspacePath:  "",
	}

	err := store.RemoveConfigField(ScopeWorkspace, "foo")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoWorkspaceConfig))
}

func TestConfigStore_HasConfigField_WorkspaceScopeGuard(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: filepath.Join(t.TempDir(), "global.json"),
		workspacePath:  "",
	}

	has := store.HasConfigField(ScopeWorkspace, "foo")
	require.False(t, has)
}

func TestConfigStore_RuntimeOverrides_Independent(t *testing.T) {
	t.Parallel()

	store1 := &ConfigStore{config: &Config{}}
	store2 := &ConfigStore{config: &Config{}}

	require.False(t, store1.Overrides().SkipPermissionRequests)
	require.False(t, store2.Overrides().SkipPermissionRequests)

	store1.Overrides().SkipPermissionRequests = true

	require.True(t, store1.Overrides().SkipPermissionRequests)
	require.False(t, store2.Overrides().SkipPermissionRequests)
}

func TestConfigStore_RuntimeOverrides_MutableViaPointer(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{config: &Config{}}
	overrides := store.Overrides()

	require.False(t, overrides.SkipPermissionRequests)

	overrides.SkipPermissionRequests = true
	require.True(t, store.Overrides().SkipPermissionRequests)
}

func TestGlobalWorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_DATA", dir)

	wsDir := GlobalWorkspaceDir()
	globalData := GlobalConfigData()

	require.Equal(t, filepath.Dir(globalData), wsDir)
	require.Equal(t, dir, wsDir)
}

func TestScope_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "global", ScopeGlobal.String())
	require.Equal(t, "workspace", ScopeWorkspace.String())
	require.Contains(t, Scope(99).String(), "Scope(99)")
}

func TestLoad_UsesGlobalModelsWithoutWorkspaceConfig(t *testing.T) {
	isolateProviderStateForTest(t)

	dir := t.TempDir()
	globalDataDir := filepath.Join(dir, "global-data")
	t.Setenv("CRUSH_GLOBAL_DATA", globalDataDir)

	globalPath := filepath.Join(globalDataDir, "crush.json")
	workspaceDir := filepath.Join(dir, ".crush")
	workspacePath := filepath.Join(workspaceDir, "crush.json")
	writeJSONFile(t, globalPath, modelSelectionTestConfig(map[string]any{
		"large": map[string]any{"provider": "test-provider", "model": "global-large"},
		"small": map[string]any{"provider": "test-provider", "model": "global-small"},
	}, nil))

	store, err := Load(dir, workspaceDir, false)
	require.NoError(t, err)
	require.Equal(t, workspacePath, store.workspacePath)
	require.NoFileExists(t, workspacePath)
	require.Equal(t, "global-large", store.config.Models[SelectedModelTypeLarge].Model)
	require.Equal(t, "global-small", store.config.Models[SelectedModelTypeSmall].Model)
}

func TestLoad_WorkspaceModelsOverrideGlobalModels(t *testing.T) {
	isolateProviderStateForTest(t)

	dir := t.TempDir()
	globalDataDir := filepath.Join(dir, "global-data")
	t.Setenv("CRUSH_GLOBAL_DATA", globalDataDir)

	globalPath := filepath.Join(globalDataDir, "crush.json")
	workspaceDir := filepath.Join(dir, ".crush")
	workspacePath := filepath.Join(workspaceDir, "crush.json")
	writeJSONFile(t, globalPath, modelSelectionTestConfig(map[string]any{
		"large": map[string]any{"provider": "test-provider", "model": "global-large"},
		"small": map[string]any{"provider": "test-provider", "model": "global-small"},
	}, nil))
	writeJSONFile(t, workspacePath, map[string]any{
		"models": map[string]any{
			"large": map[string]any{"provider": "test-provider", "model": "workspace-large"},
			"small": map[string]any{"provider": "test-provider", "model": "workspace-small"},
		},
	})

	store, err := Load(dir, workspaceDir, false)
	require.NoError(t, err)
	require.Equal(t, "workspace-large", store.config.Models[SelectedModelTypeLarge].Model)
	require.Equal(t, "workspace-small", store.config.Models[SelectedModelTypeSmall].Model)
}

func TestLoad_PartialWorkspaceModelOverrideFallsBackToGlobal(t *testing.T) {
	isolateProviderStateForTest(t)

	dir := t.TempDir()
	globalDataDir := filepath.Join(dir, "global-data")
	t.Setenv("CRUSH_GLOBAL_DATA", globalDataDir)

	globalPath := filepath.Join(globalDataDir, "crush.json")
	workspaceDir := filepath.Join(dir, ".crush")
	workspacePath := filepath.Join(workspaceDir, "crush.json")
	writeJSONFile(t, globalPath, modelSelectionTestConfig(map[string]any{
		"large": map[string]any{"provider": "test-provider", "model": "global-large"},
		"small": map[string]any{"provider": "test-provider", "model": "global-small"},
	}, nil))
	writeJSONFile(t, workspacePath, map[string]any{
		"models": map[string]any{
			"large": map[string]any{"provider": "test-provider", "model": "workspace-large"},
		},
	})

	store, err := Load(dir, workspaceDir, false)
	require.NoError(t, err)
	require.Equal(t, "workspace-large", store.config.Models[SelectedModelTypeLarge].Model)
	require.Equal(t, "global-small", store.config.Models[SelectedModelTypeSmall].Model)
}

func TestLoad_WorkspaceRecentModelsOverrideGlobalRecentModels(t *testing.T) {
	isolateProviderStateForTest(t)

	dir := t.TempDir()
	globalDataDir := filepath.Join(dir, "global-data")
	t.Setenv("CRUSH_GLOBAL_DATA", globalDataDir)

	globalPath := filepath.Join(globalDataDir, "crush.json")
	workspaceDir := filepath.Join(dir, ".crush")
	workspacePath := filepath.Join(workspaceDir, "crush.json")
	writeJSONFile(t, globalPath, modelSelectionTestConfig(map[string]any{
		"large": map[string]any{"provider": "test-provider", "model": "global-large"},
		"small": map[string]any{"provider": "test-provider", "model": "global-small"},
	}, map[string]any{
		"large": []map[string]any{{"provider": "test-provider", "model": "global-large"}},
		"small": []map[string]any{{"provider": "test-provider", "model": "global-small"}},
	}))
	writeJSONFile(t, workspacePath, map[string]any{
		"recent_models": map[string]any{
			"large": []map[string]any{{"provider": "test-provider", "model": "workspace-large"}},
		},
	})

	store, err := Load(dir, workspaceDir, false)
	require.NoError(t, err)
	require.Len(t, store.config.RecentModels[SelectedModelTypeLarge], 1)
	require.Equal(t, "workspace-large", store.config.RecentModels[SelectedModelTypeLarge][0].Model)
	require.Equal(t, "global-small", store.config.RecentModels[SelectedModelTypeSmall][0].Model)
}

// TestReloadFromDisk_WorkspaceRecentModelsOverrideGlobalRecentModels is a
// regression test ensuring ReloadFromDisk applies the same workspace
// recent_models override semantics as Load (workspace arrays replace global,
// they do not concatenate).
func TestReloadFromDisk_WorkspaceRecentModelsOverrideGlobalRecentModels(t *testing.T) {
	isolateProviderStateForTest(t)

	dir := t.TempDir()
	globalDataDir := filepath.Join(dir, "global-data")
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(dir, "no-such-global-config"))
	t.Setenv("CRUSH_GLOBAL_DATA", globalDataDir)

	globalPath := filepath.Join(globalDataDir, "crush.json")
	workspaceDir := filepath.Join(dir, ".crush")
	workspacePath := filepath.Join(workspaceDir, "crush.json")
	writeJSONFile(t, globalPath, modelSelectionTestConfig(map[string]any{
		"large": map[string]any{"provider": "test-provider", "model": "global-large"},
		"small": map[string]any{"provider": "test-provider", "model": "global-small"},
	}, map[string]any{
		"large": []map[string]any{{"provider": "test-provider", "model": "global-large"}},
		"small": []map[string]any{{"provider": "test-provider", "model": "global-small"}},
	}))
	writeJSONFile(t, workspacePath, map[string]any{
		"recent_models": map[string]any{
			"large": []map[string]any{{"provider": "test-provider", "model": "workspace-large"}},
		},
	})

	store, err := Load(dir, workspaceDir, false)
	require.NoError(t, err)
	require.Len(t, store.config.RecentModels[SelectedModelTypeLarge], 1)
	require.Equal(t, "workspace-large", store.config.RecentModels[SelectedModelTypeLarge][0].Model)

	require.NoError(t, store.ReloadFromDisk(context.Background()))
	require.Len(t, store.config.RecentModels[SelectedModelTypeLarge], 1,
		"ReloadFromDisk must replace global recent_models.large with workspace value, not concatenate")
	require.Equal(t, "workspace-large", store.config.RecentModels[SelectedModelTypeLarge][0].Model)
	require.Equal(t, "global-small", store.config.RecentModels[SelectedModelTypeSmall][0].Model)
}

func TestConfigStaleness_CleanImmediatelyAfterSnapshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create a config file
	content := []byte(`{"options": {"debug": true}}`)
	require.NoError(t, os.WriteFile(configPath, content, 0o600))

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}
	store.captureStalenessSnapshot([]string{configPath})

	result := store.ConfigStaleness()
	require.False(t, result.Dirty)
	require.Empty(t, result.Changed)
	require.Empty(t, result.Missing)
}

func TestConfigStaleness_DetectsFileContentChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create initial config file
	require.NoError(t, os.WriteFile(configPath, []byte(`{"debug": false}`), 0o600))

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}
	store.captureStalenessSnapshot([]string{configPath})

	// Modify the file
	time.Sleep(10 * time.Millisecond) // Ensure different mtime
	require.NoError(t, os.WriteFile(configPath, []byte(`{"debug": true}`), 0o600))

	result := store.ConfigStaleness()
	require.True(t, result.Dirty)
	require.Contains(t, result.Changed, configPath)
	require.Empty(t, result.Missing)
}

func TestConfigStaleness_DetectsFileDeletion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create initial config file
	require.NoError(t, os.WriteFile(configPath, []byte(`{"debug": true}`), 0o600))

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}
	store.captureStalenessSnapshot([]string{configPath})

	// Delete the file
	require.NoError(t, os.Remove(configPath))

	result := store.ConfigStaleness()
	require.True(t, result.Dirty)
	require.Empty(t, result.Changed)
	require.Contains(t, result.Missing, configPath)
}

func TestConfigStaleness_DetectsNewFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Don't create file initially
	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}
	store.captureStalenessSnapshot([]string{configPath})

	// Now create the file
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, os.WriteFile(configPath, []byte(`{"debug": true}`), 0o600))

	result := store.ConfigStaleness()
	require.True(t, result.Dirty)
	require.Contains(t, result.Changed, configPath)
	require.Empty(t, result.Missing)
}

func TestConfigStaleness_SortedOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.json")
	pathB := filepath.Join(dir, "b.json")
	pathC := filepath.Join(dir, "c.json")

	// Create all files
	for _, p := range []string{pathA, pathB, pathC} {
		require.NoError(t, os.WriteFile(p, []byte(`{}`), 0o600))
	}

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: pathA,
	}
	// Add in reverse order to test sorting
	store.captureStalenessSnapshot([]string{pathC, pathA, pathB})

	// Modify all files
	time.Sleep(10 * time.Millisecond)
	for _, p := range []string{pathA, pathB, pathC} {
		require.NoError(t, os.WriteFile(p, []byte(`{"changed": true}`), 0o600))
	}

	result := store.ConfigStaleness()
	require.True(t, result.Dirty)
	// Should be sorted alphabetically
	require.Equal(t, []string{pathA, pathB, pathC}, result.Changed)
}

func TestConfigStaleness_RefreshClearsDirtyState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create initial config file
	require.NoError(t, os.WriteFile(configPath, []byte(`{"debug": false}`), 0o600))

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}
	store.captureStalenessSnapshot([]string{configPath})

	// Modify the file
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, os.WriteFile(configPath, []byte(`{"debug": true}`), 0o600))

	// Verify dirty
	result := store.ConfigStaleness()
	require.True(t, result.Dirty)

	// Refresh snapshot
	require.NoError(t, store.RefreshStalenessSnapshot())

	// Verify clean now
	result = store.ConfigStaleness()
	require.False(t, result.Dirty)
	require.Empty(t, result.Changed)
	require.Empty(t, result.Missing)
}

// TestReloadFromDisk_UsesNewConfigValues is a regression test ensuring that
// ReloadFromDisk updates store state BEFORE running model/agent setup,
// so the new config values are used rather than stale pre-reload values.
func TestReloadFromDisk_UsesNewConfigValues(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(dir, "no-such-global-config"))
	configPath := filepath.Join(dir, "crush.json")

	// Isolate from the host's global config so only test-provided
	// providers are visible.
	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	resetProviderState()
	t.Cleanup(resetProviderState)

	// Create initial config with one model preference
	initialConfig := `{
		"models": {
			"large": {"provider": "openai", "model": "gpt-4"}
		},
		"providers": {
			"openai": {
				"api_key": "test-key",
				"models": [{"id": "gpt-4", "name": "GPT-4"}]
			}
		}
	}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0o600))

	// Load initial config properly
	store, err := Load(dir, dir, false)
	require.NoError(t, err)

	// Set globalDataPath for the test (Load doesn't set this directly)
	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})

	// Verify initial model
	require.Equal(t, "openai", store.config.Models[SelectedModelTypeLarge].Provider)
	require.Equal(t, "gpt-4", store.config.Models[SelectedModelTypeLarge].Model)

	// Modify config on disk to change model
	updatedConfig := `{
		"models": {
			"large": {"provider": "anthropic", "model": "claude-3"}
		},
		"providers": {
			"openai": {
				"api_key": "test-key",
				"models": [{"id": "gpt-4", "name": "GPT-4"}]
			},
			"anthropic": {
				"api_key": "test-key-2",
				"models": [{"id": "claude-3", "name": "Claude 3"}]
			}
		}
	}`
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, os.WriteFile(configPath, []byte(updatedConfig), 0o600))

	// Reload from disk
	ctx := context.Background()
	err = store.ReloadFromDisk(ctx)
	require.NoError(t, err)

	// Verify the NEW config values are now in effect (regression check)
	require.Equal(t, "anthropic", store.config.Models[SelectedModelTypeLarge].Provider)
	require.Equal(t, "claude-3", store.config.Models[SelectedModelTypeLarge].Model)
}

// TestSetConfigField_AutoReloads verifies that SetConfigField automatically
// reloads config into memory after writing, so subsequent reads see the new value.
func TestSetConfigField_AutoReloads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create initial config file with debug = false
	initialConfig := `{"options": {"debug": false}}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0o600))

	// Load initial config
	store, err := Load(dir, dir, false)
	require.NoError(t, err)

	// Verify initial state
	require.False(t, store.config.Options.Debug)

	// Set globalDataPath and capture snapshot for staleness tracking
	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})

	// Use SetConfigField to change debug to true
	err = store.SetConfigField(ScopeGlobal, "options.debug", true)
	require.NoError(t, err)

	// Verify in-memory state was automatically reloaded and reflects the change
	require.True(t, store.config.Options.Debug, "Expected config to auto-reload and show debug = true")

	// Verify staleness is clean after the reload
	staleness := store.ConfigStaleness()
	require.False(t, staleness.Dirty, "Expected staleness to be clean after auto-reload")
}

// TestRemoveConfigField_AutoReloads verifies that RemoveConfigField automatically
// reloads config into memory after writing.
func TestRemoveConfigField_AutoReloads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create initial config file with a custom option
	initialConfig := `{"options": {"debug": true, "custom_field": "value"}}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0o600))

	// Load initial config
	store, err := Load(dir, dir, false)
	require.NoError(t, err)

	// Set globalDataPath and capture snapshot
	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})

	// Verify the field exists initially (indirectly - store loaded successfully)
	require.True(t, store.config.Options.Debug)

	// Remove the debug field
	err = store.RemoveConfigField(ScopeGlobal, "options.debug")
	require.NoError(t, err)

	// Verify auto-reload occurred and stale state is clean
	staleness := store.ConfigStaleness()
	require.False(t, staleness.Dirty, "Expected staleness to be clean after auto-reload from RemoveConfigField")
}

// TestSetConfigField_AutoReloadSkipsWhenNoWorkingDir verifies that auto-reload
// gracefully skips when working directory is not set (e.g., during testing).
func TestSetConfigField_AutoReloadSkipsWhenNoWorkingDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create a store without working directory (like some test setups)
	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
		// workingDir is empty
	}

	// SetConfigField should succeed even without workingDir (auto-reload skips)
	err := store.SetConfigField(ScopeGlobal, "foo", "bar")
	require.NoError(t, err)

	// Verify file was still written
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "foo")
}

// TestAutoReloadDisabledDuringReload verifies that auto-reload is suppressed
// during ReloadFromDisk to prevent re-entrant/nested reload calls.
func TestAutoReloadDisabledDuringReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create initial config with a provider that will trigger config modification during reload
	// (simulating the anthropic OAuth token removal case)
	initialConfig := `{
		"providers": {
			"anthropic": {
				"api_key": "test-key",
				"oauth": {"access_token": "token", "refresh_token": "refresh"}
			}
		}
	}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0o600))

	// Load will trigger configureProviders which removes anthropic OAuth config
	// This should NOT cause infinite recursion thanks to autoReloadDisabled guard
	store, err := Load(dir, dir, false)
	require.NoError(t, err)

	// Verify the store loaded successfully and autoReloadDisabled was unset
	require.False(t, store.autoReloadDisabled)

	// Capture snapshot and verify reload also works without recursion
	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})

	// Modify file and reload - this should work without re-entrancy issues
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, os.WriteFile(configPath, []byte(`{"options": {"debug": true}}`), 0o600))

	err = store.ReloadFromDisk(context.Background())
	require.NoError(t, err)

	// Verify reload completed successfully
	require.False(t, store.autoReloadDisabled, "autoReloadDisabled should be false after ReloadFromDisk")
}

// TestSetConfigFields_AutoReloadsAtomically verifies that SetConfigFields writes
// multiple fields in a single disk write and triggers only one auto-reload,
// avoiding intermediate states where only some fields are persisted.
func TestSetConfigFields_AutoReloadsAtomically(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create initial config file.
	initialConfig := `{"options": {"debug": false}}`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0o600))

	// Load initial config.
	store, err := Load(dir, dir, false)
	require.NoError(t, err)

	// Set globalDataPath and capture snapshot.
	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})

	// Write multiple fields atomically.
	err = store.SetConfigFields(ScopeGlobal, map[string]any{
		"options.debug":  true,
		"options.custom": "hello",
	})
	require.NoError(t, err)

	// Verify both fields are reflected in memory.
	require.True(t, store.config.Options.Debug)
}

func TestConfigStore_UpdatePreferredModel_WorkspaceScopeKeepsGlobalUnchanged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global", "crush.json")
	workspacePath := filepath.Join(dir, "workspace", ".crush", "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o755))
	require.NoError(t, os.WriteFile(globalPath, []byte(`{"models":{"large":{"provider":"openai","model":"global-large"}}}`), 0o600))
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	store := &ConfigStore{
		config:         cfg,
		globalDataPath: globalPath,
		workspacePath:  workspacePath,
	}

	selected := SelectedModel{Provider: "anthropic", Model: "claude-3"}
	require.NoError(t, store.UpdatePreferredModel(ScopeWorkspace, SelectedModelTypeLarge, selected))

	// Models remain untouched in global (workspace models don't leak).
	// Recent models are written to global so they're visible across
	// projects, but global models are unchanged.
	global := readConfigJSON(t, globalPath)
	globalModels, ok := global["models"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "global-large", globalModels["large"].(map[string]any)["model"])
	globalRecents, ok := global["recent_models"].(map[string]any)
	require.True(t, ok)
	require.Len(t, globalRecents[string(SelectedModelTypeLarge)].([]any), 1)

	workspace := readConfigJSON(t, workspacePath)
	models, ok := workspace["models"].(map[string]any)
	require.True(t, ok)
	large, ok := models[string(SelectedModelTypeLarge)].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "anthropic", large["provider"])
	require.Equal(t, "claude-3", large["model"])

	recentModels, ok := workspace["recent_models"].(map[string]any)
	require.True(t, ok)
	recentLarge, ok := recentModels[string(SelectedModelTypeLarge)].([]any)
	require.True(t, ok)
	require.Len(t, recentLarge, 1)
}

func TestConfigStore_SaveModelChoicesAsDefault_EmptyStateDoesNotClobberGlobal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global", "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o755))
	require.NoError(t, os.WriteFile(globalPath, []byte(`{"models":{"large":{"provider":"openai","model":"old-global"}},"recent_models":{"large":[{"provider":"openai","model":"old-global"}]}}`), 0o600))
	beforeGlobal, err := os.ReadFile(globalPath)
	require.NoError(t, err)

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: globalPath,
	}

	err = store.SaveModelChoicesAsDefault()
	require.ErrorIs(t, err, ErrNoModelChoicesToSave)
	afterGlobal, err := os.ReadFile(globalPath)
	require.NoError(t, err)
	require.Equal(t, string(beforeGlobal), string(afterGlobal))
}

func TestConfigStore_SaveModelChoicesAsDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global", "crush.json")
	workspacePath := filepath.Join(dir, "workspace", ".crush", "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(workspacePath), 0o755))
	require.NoError(t, os.WriteFile(globalPath, []byte(`{"models":{"large":{"provider":"openai","model":"old-global"}}}`), 0o600))
	require.NoError(t, os.WriteFile(workspacePath, []byte(`{"models":{"large":{"provider":"openai","model":"workspace"}}}`), 0o600))
	beforeWorkspace, err := os.ReadFile(workspacePath)
	require.NoError(t, err)

	cfg := &Config{
		Models: map[SelectedModelType]SelectedModel{
			SelectedModelTypeLarge: {Provider: "anthropic", Model: "claude-3"},
			SelectedModelTypeSmall: {Provider: "openai", Model: "gpt-4o-mini"},
		},
		RecentModels: map[SelectedModelType][]SelectedModel{
			SelectedModelTypeLarge: {{Provider: "anthropic", Model: "claude-3"}},
			SelectedModelTypeSmall: {{Provider: "openai", Model: "gpt-4o-mini"}},
		},
	}
	store := &ConfigStore{
		config:         cfg,
		globalDataPath: globalPath,
		workspacePath:  workspacePath,
	}

	require.NoError(t, store.SaveModelChoicesAsDefault())

	afterWorkspace, err := os.ReadFile(workspacePath)
	require.NoError(t, err)
	require.Equal(t, string(beforeWorkspace), string(afterWorkspace))

	global := readConfigJSON(t, globalPath)
	models, ok := global["models"].(map[string]any)
	require.True(t, ok)
	large, ok := models[string(SelectedModelTypeLarge)].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "anthropic", large["provider"])
	require.Equal(t, "claude-3", large["model"])

	recentModels, ok := global["recent_models"].(map[string]any)
	require.True(t, ok)
	recentLarge, ok := recentModels[string(SelectedModelTypeLarge)].([]any)
	require.True(t, ok)
	require.Len(t, recentLarge, 1)
}

// TestConfigStore_SaveModelChoicesAsDefault_PreservesExistingKeys is a
// regression test ensuring SaveModelChoicesAsDefault merges per-key into the
// existing global config rather than replacing the whole `models` /
// `recent_models` objects (which would drop pre-existing entries that aren't
// in the in-memory maps).
func TestConfigStore_SaveModelChoicesAsDefault_PreservesExistingKeys(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global", "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o755))
	// Pre-existing global file already has both large and small set.
	require.NoError(t, os.WriteFile(globalPath, []byte(
		`{"models":{"large":{"provider":"openai","model":"old-large"},"small":{"provider":"openai","model":"keep-small"}},`+
			`"recent_models":{"large":[{"provider":"openai","model":"old-large"}],"small":[{"provider":"openai","model":"keep-small"}]}}`,
	), 0o600))

	// In-memory only has large; small must not be erased from disk.
	cfg := &Config{
		Models: map[SelectedModelType]SelectedModel{
			SelectedModelTypeLarge: {Provider: "anthropic", Model: "claude-3"},
		},
		RecentModels: map[SelectedModelType][]SelectedModel{
			SelectedModelTypeLarge: {{Provider: "anthropic", Model: "claude-3"}},
		},
	}
	store := &ConfigStore{
		config:         cfg,
		globalDataPath: globalPath,
	}

	require.NoError(t, store.SaveModelChoicesAsDefault())

	global := readConfigJSON(t, globalPath)
	models, ok := global["models"].(map[string]any)
	require.True(t, ok)
	large, ok := models[string(SelectedModelTypeLarge)].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "anthropic", large["provider"])
	require.Equal(t, "claude-3", large["model"])

	small, ok := models[string(SelectedModelTypeSmall)].(map[string]any)
	require.True(t, ok, "models.small must be preserved when in-memory has no small model")
	require.Equal(t, "keep-small", small["model"])

	recents, ok := global["recent_models"].(map[string]any)
	require.True(t, ok)
	recentSmall, ok := recents[string(SelectedModelTypeSmall)].([]any)
	require.True(t, ok, "recent_models.small must be preserved")
	require.Len(t, recentSmall, 1)
	require.Equal(t, "keep-small", recentSmall[0].(map[string]any)["model"])
}

func TestConfigStore_ReloadAfterWorkspaceModelWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	workspacePath := filepath.Join(dir, ".crush", "crush.json")
	initialConfig := `{
		"providers": {
			"openai": {
				"api_key": "test-key",
				"models": [
					{"id": "gpt-4", "name": "GPT-4"},
					{"id": "gpt-4o", "name": "GPT-4o"}
				]
			}
		},
		"models": {
			"large": {"provider": "openai", "model": "gpt-4"}
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"), []byte(initialConfig), 0o600))

	store, err := Load(dir, filepath.Join(dir, ".crush"), false)
	require.NoError(t, err)
	require.Equal(t, workspacePath, store.workspacePath)
	require.Equal(t, "gpt-4", store.config.Models[SelectedModelTypeLarge].Model)

	selected := SelectedModel{Provider: "openai", Model: "gpt-4o"}
	require.NoError(t, store.UpdatePreferredModel(ScopeWorkspace, SelectedModelTypeLarge, selected))
	require.Equal(t, "gpt-4o", store.config.Models[SelectedModelTypeLarge].Model)

	workspace := readConfigJSON(t, workspacePath)
	models, ok := workspace["models"].(map[string]any)
	require.True(t, ok)
	large, ok := models[string(SelectedModelTypeLarge)].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "gpt-4o", large["model"])
}

func TestLoadTokenFromDisk_ReturnsNewerToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create config file with a newer token on disk
	configContent := `{
		"providers": {
			"hyper": {
				"oauth": {
					"access_token": "newer-token-from-disk",
					"refresh_token": "refresh-abc",
					"expires_in": 3600,
					"expires_at": 9999999999
				}
			}
		}
	}`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}

	token, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.NotNil(t, token)
	require.Equal(t, "newer-token-from-disk", token.AccessToken)
	require.Equal(t, "refresh-abc", token.RefreshToken)
	require.Equal(t, 3600, token.ExpiresIn)
	require.Equal(t, int64(9999999999), token.ExpiresAt)
}

func TestLoadTokenFromDisk_ReturnsNilWhenSameToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create config file with the same token
	configContent := `{
		"providers": {
			"hyper": {
				"oauth": {
					"access_token": "same-token",
					"refresh_token": "refresh-abc",
					"expires_in": 3600,
					"expires_at": 9999999999
				}
			}
		}
	}`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}

	token, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.NotNil(t, token)
	require.Equal(t, "same-token", token.AccessToken)
}

func TestLoadTokenFromDisk_ReturnsNilWhenFileMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "nonexistent.json")

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}

	token, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.Nil(t, token)
}

func TestLoadTokenFromDisk_ReturnsNilWhenProviderMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create config file without the hyper provider
	configContent := `{"providers": {"openai": {"api_key": "test-key"}}}`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}

	token, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.Nil(t, token)
}

func TestLoadTokenFromDisk_ReturnsNilWhenOAuthMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create config file with provider but no OAuth token
	configContent := `{"providers": {"hyper": {"api_key": "test-key"}}}`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: configPath,
	}

	token, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.Nil(t, token)
}

func TestRefreshOAuthToken_UsesDiskTokenWhenDifferent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	// Create config file with a newer token on disk
	configContent := `{
		"providers": {
			"hyper": {
				"api_key": "newer-access-token",
				"oauth": {
					"access_token": "newer-access-token",
					"refresh_token": "refresh-abc",
					"expires_in": 3600,
					"expires_at": 9999999999
				}
			}
		}
	}`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	// Set up store with an older in-memory token
	oldToken := &oauth.Token{
		AccessToken:  "older-access-token",
		RefreshToken: "refresh-abc",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(-time.Hour).Unix(), // Expired
	}

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("hyper", ProviderConfig{
		ID:         "hyper",
		Name:       "Hyper",
		APIKey:     oldToken.AccessToken,
		OAuthToken: oldToken,
	})

	store := &ConfigStore{
		config: &Config{
			Providers: providers,
		},
		globalDataPath: configPath,
	}

	// Refresh should use the disk token without making an external call
	err := store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper")
	require.NoError(t, err)

	// Verify the in-memory token was updated to the disk token
	updatedConfig, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "newer-access-token", updatedConfig.APIKey)
	require.Equal(t, "newer-access-token", updatedConfig.OAuthToken.AccessToken)
	require.Equal(t, "refresh-abc", updatedConfig.OAuthToken.RefreshToken)
}
