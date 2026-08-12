package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// twoProviderConfig is a config file naming large as the given
// provider/model, with both providers available so either choice resolves.
func twoProviderConfig(provider, model string) string {
	return `{
		"models": {
			"large": {"provider": "` + provider + `", "model": "` + model + `"}
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
}

// TestModelSelectionSurvivesPeerWrite is a regression test for the model
// switching out from under the user when several Crush instances share the
// global config file. A sibling instance selecting a different model must
// not change ours, even though an unrelated config write (a token refresh,
// for example) reloads the file we both write to.
func TestModelSelectionSurvivesPeerWrite(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	resetProviderState()
	t.Cleanup(resetProviderState)

	require.NoError(t, os.WriteFile(configPath, []byte(twoProviderConfig("openai", "gpt-4")), 0o600))

	store, err := Load(dir, dir, false)
	require.NoError(t, err)
	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})

	// The user picks Claude in this instance.
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, SelectedModel{
		Provider: "anthropic",
		Model:    "claude-3",
	}))

	// A sibling instance then picks GPT-4 and writes it to the shared file.
	require.NoError(t, os.WriteFile(configPath, []byte(twoProviderConfig("openai", "gpt-4")), 0o600))

	// Any reload (here explicit; in practice triggered by an unrelated
	// write such as an OAuth token refresh) must leave our choice alone.
	require.NoError(t, store.ReloadFromDisk(context.Background()))

	large := store.Config().Models[SelectedModelTypeLarge]
	require.Equal(t, "anthropic", large.Provider)
	require.Equal(t, "claude-3", large.Model)
}

// TestModelSelectionYieldsToDiskWhenUnchosen verifies the other half of the
// rule: a model type this instance never selected still follows the config
// file, so external edits and `crush login` defaults keep working.
func TestModelSelectionYieldsToDiskWhenUnchosen(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	resetProviderState()
	t.Cleanup(resetProviderState)

	require.NoError(t, os.WriteFile(configPath, []byte(twoProviderConfig("openai", "gpt-4")), 0o600))

	store, err := Load(dir, dir, false)
	require.NoError(t, err)
	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})

	require.NoError(t, os.WriteFile(configPath, []byte(twoProviderConfig("anthropic", "claude-3")), 0o600))
	require.NoError(t, store.ReloadFromDisk(context.Background()))

	large := store.Config().Models[SelectedModelTypeLarge]
	require.Equal(t, "anthropic", large.Provider)
	require.Equal(t, "claude-3", large.Model)
}
