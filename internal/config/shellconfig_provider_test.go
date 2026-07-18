package config_test

import (
	"slices"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestShellConfigProviderAddAndModel(t *testing.T) {
	store := loadCrushSh(t, `provider add myllm \
  --type openai-compat \
  --base-url "http://localhost:1234/v1" \
  --api-key "sk-test"
model add myllm/foo-1 --name "Foo 1" --context-window 8000
model large myllm/foo-1`)

	cfg := store.Config()

	p, ok := cfg.Providers.Get("myllm")
	require.True(t, ok, "myllm provider should be configured")
	require.Equal(t, "sk-test", p.APIKey)
	require.Equal(t, "http://localhost:1234/v1", p.BaseURL)
	require.True(
		t,
		slices.ContainsFunc(p.Models, func(m catwalk.Model) bool { return m.ID == "foo-1" }),
		"custom model foo-1 should be in the provider catalog",
	)

	large := cfg.Models[config.SelectedModelTypeLarge]
	require.Equal(t, "myllm", large.Provider)
	require.Equal(t, "foo-1", large.Model)
}

func TestShellConfigProviderRemove(t *testing.T) {
	// Both providers get a model so they survive provider configuration
	// (model-less providers are dropped); the only difference is the remove.
	store := loadCrushSh(t, `provider add keepme --type openai-compat --base-url "http://localhost:1/v1" --api-key k
model add keepme/m1 --name M1
provider add dropme --type openai-compat --base-url "http://localhost:2/v1" --api-key k
model add dropme/m2 --name M2
provider remove dropme`)

	_, keep := store.Config().Providers.Get("keepme")
	_, drop := store.Config().Providers.Get("dropme")
	require.True(t, keep, "keepme should remain")
	require.False(t, drop, "dropme should be gone after remove")
}
