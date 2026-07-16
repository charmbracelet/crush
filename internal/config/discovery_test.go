package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConfigStore_DiscoverModels verifies the background discovery path:
// a custom provider with no models is dropped on initial load, then
// reappears with discovered models after DiscoverModels populates the
// in-memory cache and reloads.
func TestConfigStore_DiscoverModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [
			{"id": "auto-model-a", "object": "model"},
			{"id": "auto-model-b", "object": "model"}
		]}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	resetProviderState()
	t.Cleanup(resetProviderState)

	cfgJSON := `{
		"providers": {
			"custom": {
				"type": "openai-compat",
				"api_key": "test-key",
				"base_url": "` + server.URL + `/v1"
			}
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"), []byte(cfgJSON), 0o600))

	store, err := Load(t.Context(), dir, dir, false)
	require.NoError(t, err)

	// No models yet and nothing cached, so the provider is dropped.
	_, exists := store.Config().Providers.Get("custom")
	require.False(t, exists, "provider should be absent before discovery runs")

	// Run discovery: it fetches from the server, caches results, reloads.
	changed, err := store.DiscoverModels(t.Context())
	require.NoError(t, err)
	require.True(t, changed, "discovery should report changes")

	p, exists := store.Config().Providers.Get("custom")
	require.True(t, exists, "provider should reappear after discovery")
	require.Len(t, p.Models, 2)
	require.Equal(t, "auto-model-a", p.Models[0].ID)
	require.Equal(t, "auto-model-b", p.Models[1].ID)
}

// TestConfigStore_HasPendingModelDiscovery confirms the pending check
// tracks whether any custom provider still needs discovery.
func TestConfigStore_HasPendingModelDiscovery(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	resetProviderState()
	t.Cleanup(resetProviderState)

	cfgJSON := `{
		"providers": {
			"custom": {
				"type": "openai-compat",
				"api_key": "test-key",
				"base_url": "http://127.0.0.1:1/v1"
			}
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"), []byte(cfgJSON), 0o600))

	store, err := Load(t.Context(), dir, dir, false)
	require.NoError(t, err)
	require.True(t, store.HasPendingModelDiscovery())
}

// TestConfigStore_DiscoverModels_RestoresSelectedModel guards against the
// worst regression of deferring discovery: if the user's selected model
// belongs to a provider that needs discovery, Load must not persist a
// fallback that would clobber the choice. The selection is served from the
// fallback in memory until discovery runs, then restored on reload.
func TestConfigStore_DiscoverModels_RestoresSelectedModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "slow-model", "object": "model"}]}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	resetProviderState()
	t.Cleanup(resetProviderState)

	configPath := filepath.Join(dir, "crush.json")
	cfgJSON := `{
		"models": {"large": {"provider": "slow", "model": "slow-model"}},
		"providers": {
			"good": {
				"type": "openai-compat",
				"api_key": "test-key",
				"base_url": "https://api.good.com/v1",
				"models": [{"id": "good-model", "name": "good-model"}]
			},
			"slow": {
				"type": "openai-compat",
				"api_key": "test-key",
				"base_url": "` + server.URL + `/v1"
			}
		}
	}`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgJSON), 0o600))

	store, err := Load(t.Context(), dir, dir, false)
	require.NoError(t, err)

	// Before discovery: the slow provider is dropped, so the selection
	// falls back in memory to the working provider.
	require.Equal(t, "good", store.Config().Models[SelectedModelTypeLarge].Provider,
		"selection should fall back in memory while slow provider is pending")

	// The on-disk config must NOT have been rewritten to the fallback.
	onDisk, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(onDisk), `"provider": "slow"`,
		"the user's selection must survive on disk")

	// After discovery the slow provider gains models and the reload
	// restores the user's original selection.
	changed, err := store.DiscoverModels(t.Context())
	require.NoError(t, err)
	require.True(t, changed)

	require.Equal(t, "slow", store.Config().Models[SelectedModelTypeLarge].Provider,
		"original selection should be restored after discovery")
	require.Equal(t, "slow-model", store.Config().Models[SelectedModelTypeLarge].Model)
}
