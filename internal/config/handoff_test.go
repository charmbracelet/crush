package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

func TestConfigureSelectedModels_DefaultsHandoffToLarge(t *testing.T) {
	t.Parallel()

	knownProviders := []catwalk.Provider{
		{
			ID:                  "openai",
			Name:                "OpenAI",
			APIEndpoint:         "https://api.openai.com/v1",
			DefaultLargeModelID: "gpt-5",
			DefaultSmallModelID: "gpt-5-mini",
			Models: []catwalk.Model{
				{ID: "gpt-5", DefaultMaxTokens: 32000},
				{ID: "gpt-5-mini", DefaultMaxTokens: 16000},
			},
		},
	}

	cfg := &Config{
		Providers: csync.NewMap[string, ProviderConfig](),
	}
	cfg.setDefaults(t.TempDir(), "")
	cfg.Providers.Set("openai", ProviderConfig{
		ID:     "openai",
		Name:   "OpenAI",
		APIKey: "test-key",
		Models: []catwalk.Model{
			{ID: "gpt-5", DefaultMaxTokens: 32000},
			{ID: "gpt-5-mini", DefaultMaxTokens: 16000},
		},
	})

	resolver := NewEnvironmentVariableResolver(env.NewFromMap(map[string]string{}))
	require.NoError(t, cfg.configureProviders(testStore(cfg), env.NewFromMap(map[string]string{}), resolver, knownProviders))
	require.NoError(t, configureSelectedModels(testStore(cfg), knownProviders))

	require.Equal(t, cfg.Models[SelectedModelTypeLarge], cfg.Models[SelectedModelTypeHandoff])
	require.Equal(t, cfg.Models[SelectedModelTypeHandoff], cfg.Models[SelectedModelTypeAutoClassifierReasoning])
	require.Equal(t, cfg.Models[SelectedModelTypeSmall], cfg.Models[SelectedModelTypeAutoClassifierFast])
}

func TestConfigureSelectedModels_UsesExplicitHandoff(t *testing.T) {
	t.Parallel()

	knownProviders := []catwalk.Provider{
		{
			ID:                  "openai",
			Name:                "OpenAI",
			APIEndpoint:         "https://api.openai.com/v1",
			DefaultLargeModelID: "gpt-5",
			DefaultSmallModelID: "gpt-5-mini",
			Models: []catwalk.Model{
				{ID: "gpt-5", DefaultMaxTokens: 32000},
				{ID: "gpt-5-mini", DefaultMaxTokens: 16000},
				{ID: "gpt-4.1", DefaultMaxTokens: 12000},
			},
		},
	}

	cfg := &Config{
		Providers: csync.NewMap[string, ProviderConfig](),
		Models: map[SelectedModelType]SelectedModel{
			SelectedModelTypeHandoff: {
				Provider: "openai",
				Model:    "gpt-4.1",
			},
		},
	}
	cfg.setDefaults(t.TempDir(), "")
	cfg.Providers.Set("openai", ProviderConfig{
		ID:     "openai",
		Name:   "OpenAI",
		APIKey: "test-key",
		Models: []catwalk.Model{
			{ID: "gpt-5", DefaultMaxTokens: 32000},
			{ID: "gpt-5-mini", DefaultMaxTokens: 16000},
			{ID: "gpt-4.1", DefaultMaxTokens: 12000},
		},
	})

	envMap := env.NewFromMap(map[string]string{})
	resolver := NewEnvironmentVariableResolver(envMap)
	require.NoError(t, cfg.configureProviders(testStore(cfg), envMap, resolver, knownProviders))
	require.NoError(t, configureSelectedModels(testStore(cfg), knownProviders))

	require.Equal(t, "gpt-4.1", cfg.Models[SelectedModelTypeHandoff].Model)
	require.Equal(t, "openai", cfg.Models[SelectedModelTypeHandoff].Provider)
}

func TestConfigureSelectedModels_UsesExplicitAutoClassifier(t *testing.T) {
	t.Parallel()

	knownProviders := []catwalk.Provider{
		{
			ID:                  "openai",
			Name:                "OpenAI",
			APIEndpoint:         "https://api.openai.com/v1",
			DefaultLargeModelID: "gpt-5",
			DefaultSmallModelID: "gpt-5-mini",
			Models: []catwalk.Model{
				{ID: "gpt-5", DefaultMaxTokens: 32000},
				{ID: "gpt-5-mini", DefaultMaxTokens: 16000},
				{ID: "gpt-4.1-mini", DefaultMaxTokens: 8000},
			},
		},
	}

	temperature := 0.0
	cfg := &Config{
		Providers: csync.NewMap[string, ProviderConfig](),
		Models: map[SelectedModelType]SelectedModel{
			SelectedModelTypeAutoClassifierFast: {
				Provider:    "openai",
				Model:       "gpt-4.1-mini",
				Temperature: &temperature,
			},
		},
	}
	cfg.setDefaults(t.TempDir(), "")
	cfg.Providers.Set("openai", ProviderConfig{
		ID:     "openai",
		Name:   "OpenAI",
		APIKey: "test-key",
		Models: []catwalk.Model{
			{ID: "gpt-5", DefaultMaxTokens: 32000},
			{ID: "gpt-5-mini", DefaultMaxTokens: 16000},
			{ID: "gpt-4.1-mini", DefaultMaxTokens: 8000},
		},
	})

	envMap := env.NewFromMap(map[string]string{})
	resolver := NewEnvironmentVariableResolver(envMap)
	require.NoError(t, cfg.configureProviders(testStore(cfg), envMap, resolver, knownProviders))
	require.NoError(t, configureSelectedModels(testStore(cfg), knownProviders))

	require.Equal(t, "gpt-4.1-mini", cfg.Models[SelectedModelTypeAutoClassifierFast].Model)
	require.Equal(t, "openai", cfg.Models[SelectedModelTypeAutoClassifierFast].Provider)
	require.NotNil(t, cfg.Models[SelectedModelTypeAutoClassifierFast].Temperature)
	require.Equal(t, temperature, *cfg.Models[SelectedModelTypeAutoClassifierFast].Temperature)
}
