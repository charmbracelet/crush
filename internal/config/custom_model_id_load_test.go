package config

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/catwalk/pkg/embedded"
	"github.com/stretchr/testify/require"
)

// TestLoadHonorsDeclaredCustomModelIDOverEmbeddedCatalog pins the #2649
// crush.json shape end-to-end: with disable_provider_auto_update and a
// declared short id that collides with the embedded catalog's dated default
// small model, Load keeps the declared short id on the small slot and the
// novel id on the large slot.
func TestLoadHonorsDeclaredCustomModelIDOverEmbeddedCatalog(t *testing.T) {
	const (
		shortID = "claude-haiku-4-5"
		datedID = "claude-haiku-4-5-20251001"
		novelID = "my-truly-novel-model"
	)

	// Preconditions: the real embedded Anthropic catalog still ships the
	// dated alias as default small. If that ever changes, this pin needs a
	// fresh colliding pair rather than a silent pass.
	var foundDated bool
	for _, p := range embedded.GetAll() {
		if string(p.ID) != "anthropic" {
			continue
		}
		require.Equal(t, datedID, p.DefaultSmallModelID,
			"embedded anthropic default small must still be the dated haiku alias")
		for _, m := range p.Models {
			if m.ID == datedID {
				foundDated = true
				break
			}
		}
	}
	require.True(t, foundDated, "embedded anthropic catalog must contain %s", datedID)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")
	crushJSON := `{
  "$schema": "https://charm.land/crush.json",
  "options": { "disable_provider_auto_update": true },
  "providers": {
    "anthropic": {
      "base_url": "https://proxy.example/anthropic",
      "api_key": "test-key",
      "models": [
        {
          "id": "claude-haiku-4-5",
          "name": "Custom short haiku",
          "cost_per_1m_in": 1,
          "cost_per_1m_out": 5,
          "cost_per_1m_in_cached": 1.25,
          "cost_per_1m_out_cached": 0.1,
          "context_window": 200000,
          "default_max_tokens": 64000,
          "can_reason": true,
          "supports_attachments": true
        },
        {
          "id": "my-truly-novel-model",
          "name": "Novel",
          "cost_per_1m_in": 1,
          "cost_per_1m_out": 5,
          "cost_per_1m_in_cached": 1.25,
          "cost_per_1m_out_cached": 0.1,
          "context_window": 200000,
          "default_max_tokens": 64000,
          "can_reason": true,
          "supports_attachments": true
        }
      ]
    }
  },
  "models": {
    "large": { "model": "my-truly-novel-model", "provider": "anthropic" },
    "small": { "model": "claude-haiku-4-5", "provider": "anthropic" }
  }
}`
	require.NoError(t, os.WriteFile(configPath, []byte(crushJSON), 0o600))

	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	resetProviderState()
	t.Cleanup(resetProviderState)

	store, err := Load(dir, dir, false)
	require.NoError(t, err)
	cfg := store.Config()

	small := cfg.Models[SelectedModelTypeSmall]
	require.Equal(t, "anthropic", small.Provider)
	require.Equal(t, shortID, small.Model, "declared short id must win over catalog dated alias")

	large := cfg.Models[SelectedModelTypeLarge]
	require.Equal(t, "anthropic", large.Provider)
	require.Equal(t, novelID, large.Model, "novel id must pass through unchanged")

	gotShort := cfg.GetModel("anthropic", shortID)
	require.NotNil(t, gotShort)
	require.Equal(t, shortID, gotShort.ID)
	require.Equal(t, "Custom short haiku", gotShort.Name)

	gotNovel := cfg.GetModel("anthropic", novelID)
	require.NotNil(t, gotNovel)
	require.Equal(t, novelID, gotNovel.ID)

	// Dated catalog entry remains available under its own id; it must not
	// replace the declared short id on either slot.
	gotDated := cfg.GetModel("anthropic", datedID)
	require.NotNil(t, gotDated, "catalog dated alias should still be mergeable by exact id")
	require.Equal(t, datedID, gotDated.ID)
	require.NotEqual(t, datedID, small.Model)
	require.NotEqual(t, datedID, large.Model)
}
