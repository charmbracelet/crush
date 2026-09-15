package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

func newConfigWithProviders(t *testing.T, providers map[string][]string) *Config {
	t.Helper()

	pMap := csync.NewMap[string, ProviderConfig]()
	for id, modelIDs := range providers {
		models := make([]catwalk.Model, 0, len(modelIDs))
		for _, mid := range modelIDs {
			models = append(models, catwalk.Model{ID: mid})
		}
		pMap.Set(id, ProviderConfig{ID: id, Models: models})
	}
	return &Config{Providers: pMap}
}

func TestConfig_IsKnownModelID(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProviders(t, map[string][]string{
		"openai":    {"gpt-4o", "gpt-4o-mini"},
		"anthropic": {"claude-opus-4-7", "claude-sonnet-4-6"},
	})

	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"empty_string", "", false},
		{"unknown_id", "imaginary-99", false},
		{"first_provider_first_model", "gpt-4o", true},
		{"first_provider_second_model", "gpt-4o-mini", true},
		{"second_provider", "claude-opus-4-7", true},
		{"case_sensitive", "GPT-4o", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, cfg.IsKnownModelID(tt.id))
		})
	}
}

func TestConfig_IsKnownModelID_IgnoresDisabledProvider(t *testing.T) {
	t.Parallel()

	pMap := csync.NewMap[string, ProviderConfig]()
	pMap.Set("disabled-provider", ProviderConfig{
		ID:      "disabled-provider",
		Disable: true,
		Models:  []catwalk.Model{{ID: "only-here-model"}},
	})
	cfg := &Config{Providers: pMap}

	require.False(t, cfg.IsKnownModelID("only-here-model"))
}

func TestConfig_IsKnownModelID_NoProviders(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProviders(t, nil)
	require.False(t, cfg.IsKnownModelID("gpt-4o"))
	require.False(t, cfg.IsKnownModelID(""))
}

func TestConfig_IsKnownModel(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProviders(t, map[string][]string{
		"openai":    {"gpt-4o", "gpt-4o-mini"},
		"anthropic": {"claude-opus-4-7", "claude-sonnet-4-6"},
	})

	tests := []struct {
		name     string
		provider string
		modelID  string
		want     bool
	}{
		{
			name:     "empty_provider_scans_all_known_id",
			provider: "",
			modelID:  "gpt-4o",
			want:     true,
		},
		{
			name:     "empty_provider_scans_all_unknown_id",
			provider: "",
			modelID:  "imaginary-99",
			want:     false,
		},
		{
			name:     "empty_provider_empty_model",
			provider: "",
			modelID:  "",
			want:     false,
		},
		{
			name:     "specific_provider_model_match",
			provider: "openai",
			modelID:  "gpt-4o",
			want:     true,
		},
		{
			name:     "specific_provider_model_no_match",
			provider: "openai",
			modelID:  "claude-opus-4-7",
			want:     false,
		},
		{
			name:     "specific_provider_unknown_model",
			provider: "openai",
			modelID:  "imaginary-99",
			want:     false,
		},
		{
			name:     "unknown_provider",
			provider: "nonexistent",
			modelID:  "gpt-4o",
			want:     false,
		},
		{
			name:     "second_provider_specific",
			provider: "anthropic",
			modelID:  "claude-opus-4-7",
			want:     true,
		},
		{
			name:     "case_sensitive_provider",
			provider: "openai",
			modelID:  "GPT-4o",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, cfg.IsKnownModel(tt.provider, tt.modelID))
		})
	}
}

func TestConfig_IsKnownModel_NoProviders(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProviders(t, nil)
	require.False(t, cfg.IsKnownModel("", "gpt-4o"))
	require.False(t, cfg.IsKnownModel("openai", "gpt-4o"))
	require.False(t, cfg.IsKnownModel("", ""))
}

// newConfigWithDisabledProvider builds a *Config with a single provider
// offering modelIDs, marked Disable: true.
func newConfigWithDisabledProvider(t *testing.T, providerID string, modelIDs ...string) *Config {
	t.Helper()

	models := make([]catwalk.Model, 0, len(modelIDs))
	for _, mid := range modelIDs {
		models = append(models, catwalk.Model{ID: mid})
	}
	pMap := csync.NewMap[string, ProviderConfig]()
	pMap.Set(providerID, ProviderConfig{ID: providerID, Disable: true, Models: models})
	return &Config{Providers: pMap}
}

// TestConfig_IsKnownModel_IgnoresDisabledProviderWithExplicitProvider is the
// regression test for IsKnownModel(provider, modelID) returning true for a
// model offered by a provider the user has explicitly disabled, when the
// caller names that provider directly. IsKnownModel delegates to GetModel,
// which never checks ProviderConfig.Disable — unlike the sibling
// IsModelAvailable, which does.
func TestConfig_IsKnownModel_IgnoresDisabledProviderWithExplicitProvider(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithDisabledProvider(t, "openai", "gpt-4o")

	require.False(t, cfg.IsKnownModel("openai", "gpt-4o"),
		"a disabled provider's model must not be reported known when the provider is named explicitly")
}
