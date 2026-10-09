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

func TestConfig_FindModelProvider(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProviders(t, map[string][]string{
		"openai":    {"gpt-4o", "gpt-4o-mini", "shared"},
		"azure":     {"shared"},
		"anthropic": {"claude-opus-4-7", "claude-sonnet-4-6"},
	})

	tests := []struct {
		name         string
		provider     string
		modelID      string
		wantProvider string
		wantErr      string
	}{
		{name: "empty_model", wantErr: "model id is empty"},
		{name: "unique_id_no_provider", modelID: "gpt-4o", wantProvider: "openai"},
		{name: "second_provider_no_provider", modelID: "claude-opus-4-7", wantProvider: "anthropic"},
		{name: "unknown_id_no_provider", modelID: "imaginary-99", wantErr: "not offered by any configured provider"},
		{name: "case_sensitive", modelID: "GPT-4o", wantErr: "not offered by any configured provider"},
		{name: "ambiguous_id_no_provider", modelID: "shared", wantErr: `offered by multiple providers (azure, openai); set provider`},
		{name: "ambiguous_id_with_provider", provider: "azure", modelID: "shared", wantProvider: "azure"},
		{name: "specific_provider_match", provider: "openai", modelID: "gpt-4o", wantProvider: "openai"},
		{name: "specific_provider_wrong_model", provider: "openai", modelID: "claude-opus-4-7", wantErr: `not offered by provider "openai"`},
		{name: "unknown_provider", provider: "nonexistent", modelID: "gpt-4o", wantErr: "not configured or is disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, m, err := cfg.FindModelProvider(tt.provider, tt.modelID)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.ErrorContains(t, cfg.ValidateModel(tt.provider, tt.modelID), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, cfg.ValidateModel(tt.provider, tt.modelID))
			require.Equal(t, tt.wantProvider, p.ID)
			require.Equal(t, tt.modelID, m.ID)
		})
	}
}

// TestConfig_FindModelProvider_PrefersSelectedProvider verifies an id
// several providers offer resolves to the selected large model's provider,
// then the small one's, and stays ambiguous when neither offers it.
func TestConfig_FindModelProvider_PrefersSelectedProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		large, small string
		wantProvider string
	}{
		{name: "large_provider", large: "openai", small: "anthropic", wantProvider: "openai"},
		{name: "small_provider", large: "anthropic", small: "azure", wantProvider: "azure"},
		{name: "large_wins_over_small", large: "azure", small: "openai", wantProvider: "azure"},
		{name: "neither_selected", large: "anthropic", small: "anthropic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := newConfigWithProviders(t, map[string][]string{
				"openai":    {"shared"},
				"azure":     {"shared"},
				"anthropic": {"claude-opus-4-7"},
			})
			cfg.Models = map[SelectedModelType]SelectedModel{
				SelectedModelTypeLarge: {Provider: tt.large},
				SelectedModelTypeSmall: {Provider: tt.small},
			}
			p, _, err := cfg.FindModelProvider("", "shared")
			if tt.wantProvider == "" {
				require.ErrorContains(t, err, "offered by multiple providers")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantProvider, p.ID)
		})
	}
}

func TestConfig_FindModelProvider_SimilarHint(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProviders(t, map[string][]string{
		"openai":    {"gpt-4o", "gpt-4o-mini"},
		"anthropic": {"claude-opus-4-7", "claude-sonnet-4-6"},
	})

	_, _, err := cfg.FindModelProvider("", "Sonnet")
	require.EqualError(t, err, `model "Sonnet" is not offered by any configured provider; similar: claude-sonnet-4-6`)

	_, _, err = cfg.FindModelProvider("openai", "gpt-4")
	require.EqualError(t, err, `model "gpt-4" is not offered by provider "openai"; similar: gpt-4o, gpt-4o-mini`)

	_, _, err = cfg.FindModelProvider("", "imaginary-99")
	require.EqualError(t, err, `model "imaginary-99" is not offered by any configured provider`)
}

func TestConfig_FindModelProvider_NoProviders(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProviders(t, nil)
	require.Error(t, cfg.ValidateModel("", "gpt-4o"))
	require.Error(t, cfg.ValidateModel("openai", "gpt-4o"))
	require.Error(t, cfg.ValidateModel("", ""))
}

// TestConfig_FindModelProvider_SubscriptionCatalogs covers models that only
// exist in a provider's ChatGPT or Grok subscription catalog: they must
// resolve with and without an explicit provider, like GetModel.
func TestConfig_FindModelProvider_SubscriptionCatalogs(t *testing.T) {
	t.Parallel()

	pMap := csync.NewMap[string, ProviderConfig]()
	pMap.Set("openai", ProviderConfig{ID: "openai", ChatGPTModels: []catwalk.Model{{ID: "gpt-sub"}}})
	pMap.Set("xai", ProviderConfig{ID: "xai", GrokModels: []catwalk.Model{{ID: "grok-sub"}}})
	cfg := &Config{Providers: pMap}

	for _, tc := range []struct{ provider, model string }{
		{"", "gpt-sub"}, {"openai", "gpt-sub"}, {"", "grok-sub"}, {"xai", "grok-sub"},
	} {
		_, m, err := cfg.FindModelProvider(tc.provider, tc.model)
		require.NoError(t, err, "provider=%q model=%q", tc.provider, tc.model)
		require.Equal(t, tc.model, m.ID)
	}
}

// TestConfig_FindModelProvider_IgnoresDisabledProvider ensures a disabled
// provider's models are never resolved, whether scanned or named explicitly.
func TestConfig_FindModelProvider_IgnoresDisabledProvider(t *testing.T) {
	t.Parallel()

	pMap := csync.NewMap[string, ProviderConfig]()
	pMap.Set("openai", ProviderConfig{ID: "openai", Disable: true, Models: []catwalk.Model{{ID: "gpt-4o"}}})
	cfg := &Config{Providers: pMap}

	require.Error(t, cfg.ValidateModel("", "gpt-4o"))
	require.Error(t, cfg.ValidateModel("openai", "gpt-4o"))
}
