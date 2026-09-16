package config

import (
	"context"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

// TestDeclaredCustomModelIDWinsOverCatalogDatedAlias pins #2649:
// when providers.<name>.models[] declares a short id that sits alongside a
// catalog dated alias (e.g. claude-haiku-4-5 vs claude-haiku-4-5-20251001),
// configureProviders keeps the declared entry first (exact-ID dedup) and
// resolveSelectedModels retains the declared short id — it must not fall
// back to the dated catalog default.
func TestDeclaredCustomModelIDWinsOverCatalogDatedAlias(t *testing.T) {
	const (
		shortID = "claude-haiku-4-5"
		datedID = "claude-haiku-4-5-20251001"
		novelID = "my-truly-novel-model"
	)

	knownProviders := []catwalk.Provider{
		{
			ID:                  catwalk.InferenceProviderAnthropic,
			Name:                "Anthropic",
			APIKey:              "$ANTHROPIC_API_KEY",
			APIEndpoint:         "https://api.anthropic.com/v1",
			DefaultLargeModelID: "claude-opus-4-20250514",
			DefaultSmallModelID: datedID,
			Models: []catwalk.Model{
				{ID: "claude-opus-4-20250514", Name: "Opus", DefaultMaxTokens: 32000},
				{ID: datedID, Name: "Haiku dated", DefaultMaxTokens: 64000},
			},
		},
	}

	cfg := &Config{
		Providers: csync.NewMap[string, ProviderConfig](),
		Models: map[SelectedModelType]SelectedModel{
			SelectedModelTypeLarge: {Provider: "anthropic", Model: novelID},
			SelectedModelTypeSmall: {Provider: "anthropic", Model: shortID},
		},
	}
	cfg.Providers.Set("anthropic", ProviderConfig{
		APIKey:  "test-key",
		BaseURL: "https://proxy.example/anthropic",
		Models: []catwalk.Model{
			{
				ID:               shortID,
				Name:             "Custom short haiku",
				DefaultMaxTokens: 64000,
			},
			{
				ID:               novelID,
				Name:             "Novel",
				DefaultMaxTokens: 64000,
			},
		},
	})
	cfg.setDefaults("/tmp", "")

	envMap := env.NewFromMap(map[string]string{
		"ANTHROPIC_API_KEY": "test-key",
	})
	resolver := NewShellVariableResolver(envMap)
	err := cfg.configureProviders(context.Background(), testStore(cfg), envMap, resolver, knownProviders)
	require.NoError(t, err)

	pc, ok := cfg.Providers.Get("anthropic")
	require.True(t, ok)
	require.GreaterOrEqual(t, len(pc.Models), 2)

	// Declared short id is first and exact-ID dedup does not collapse it into
	// the dated catalog entry (they are distinct ids).
	require.Equal(t, shortID, pc.Models[0].ID)
	require.Equal(t, "Custom short haiku", pc.Models[0].Name)

	var sawDated, sawNovel bool
	for _, m := range pc.Models {
		switch m.ID {
		case datedID:
			sawDated = true
		case novelID:
			sawNovel = true
		}
	}
	require.True(t, sawDated, "catalog dated alias should still be present under its own id")
	require.True(t, sawNovel, "novel declared id should pass through unchanged")

	gotShort := cfg.GetModel("anthropic", shortID)
	require.NotNil(t, gotShort, "declared short id must resolve")
	require.Equal(t, shortID, gotShort.ID)
	require.Equal(t, "Custom short haiku", gotShort.Name)

	gotNovel := cfg.GetModel("anthropic", novelID)
	require.NotNil(t, gotNovel)
	require.Equal(t, novelID, gotNovel.ID)

	resolved, err := resolveSelectedModels(cfg, knownProviders)
	require.NoError(t, err)
	require.False(t, resolved.SmallFallback, "short id must not fall back to catalog default")
	require.False(t, resolved.LargeFallback, "novel id must not fall back")
	require.Equal(t, shortID, resolved.Small.Model)
	require.Equal(t, "anthropic", resolved.Small.Provider)
	require.Equal(t, novelID, resolved.Large.Model)
	require.Equal(t, "anthropic", resolved.Large.Provider)
}
