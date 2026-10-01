package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth/antigravity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func subscriptionConfig(models ...string) *Config {
	entries := make([]catwalk.Model, 0, len(models))
	for _, id := range models {
		entries = append(entries, catwalk.Model{
			ID:                     id,
			Name:                   id,
			ContextWindow:          1048576,
			DefaultMaxTokens:       8192,
			CanReason:              true,
			ReasoningLevels:        []string{"low", "medium", "high"},
			DefaultReasoningEffort: "medium",
		})
	}
	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set(antigravity.ProviderID, ProviderConfig{ID: antigravity.ProviderID, Models: entries})
	return &Config{
		Providers: providers,
		Models:    map[SelectedModelType]SelectedModel{},
	}
}

// Builds and earlier releases selected a tier directly, e.g.
// `gemini-3.8-flash-high`. The catalog no longer has those ids, so the choice
// has to move onto the reasoning selector instead of falling back elsewhere.
func TestMigrateSubscriptionModelSelections(t *testing.T) {
	cfg := subscriptionConfig("gemini-3.8-flash", "gemini-3.1-pro", "gemini-3.1-flash-lite")
	cfg.Models[SelectedModelTypeLarge] = SelectedModel{
		Model:    "gemini-3.8-flash-high",
		Provider: antigravity.ProviderID,
	}
	cfg.Models[SelectedModelTypeSmall] = SelectedModel{
		Model:    "gemini-3.1-flash-lite",
		Provider: antigravity.ProviderID,
	}
	cfg.RecentModels = map[SelectedModelType][]SelectedModel{
		SelectedModelTypeLarge: {
			{Model: "gemini-3.8-flash-high", Provider: antigravity.ProviderID},
			{Model: "gemini-3.8-flash-low", Provider: antigravity.ProviderID},
			{Model: "gemini-3.1-pro-low", Provider: antigravity.ProviderID},
			{Model: "gemma4:e4b", Provider: "ollama"},
		},
	}

	cfg.migrateSubscriptionModelSelections()

	large := cfg.Models[SelectedModelTypeLarge]
	assert.Equal(t, "gemini-3.8-flash", large.Model)
	assert.Equal(t, "high", large.ReasoningEffort)

	// A selection that still resolves is untouched, effort included.
	small := cfg.Models[SelectedModelTypeSmall]
	assert.Equal(t, "gemini-3.1-flash-lite", small.Model)
	assert.Empty(t, small.ReasoningEffort)

	// Two tiers of one family collapse into one recent entry; foreign providers
	// are left alone.
	require.Len(t, cfg.RecentModels[SelectedModelTypeLarge], 3)
	assert.Equal(t, SelectedModel{Model: "gemini-3.8-flash", Provider: antigravity.ProviderID, ReasoningEffort: "high"},
		cfg.RecentModels[SelectedModelTypeLarge][0])
	assert.Equal(t, "gemini-3.1-pro", cfg.RecentModels[SelectedModelTypeLarge][1].Model)
	assert.Equal(t, "low", cfg.RecentModels[SelectedModelTypeLarge][1].ReasoningEffort)
	assert.Equal(t, "gemma4:e4b", cfg.RecentModels[SelectedModelTypeLarge][2].Model)
}

// An explicitly chosen effort outranks what the retired id implied, and a model
// the new catalog does not have is not ours to rename.
func TestMigrateSubscriptionModelSelectionsLeavesAmbiguityAlone(t *testing.T) {
	cfg := subscriptionConfig("gemini-3.8-flash")
	cfg.Models[SelectedModelTypeLarge] = SelectedModel{
		Model:           "gemini-3.8-flash-high",
		Provider:        antigravity.ProviderID,
		ReasoningEffort: "low",
	}
	cfg.RecentModels = map[SelectedModelType][]SelectedModel{}
	cfg.migrateSubscriptionModelSelections()
	assert.Equal(t, "low", cfg.Models[SelectedModelTypeLarge].ReasoningEffort)
	assert.Equal(t, "gemini-3.8-flash", cfg.Models[SelectedModelTypeLarge].Model)

	// A tier whose family the account does not grant stays as it was.
	other := subscriptionConfig("gemini-3.1-pro")
	other.Models[SelectedModelTypeLarge] = SelectedModel{
		Model:    "gemini-3.8-flash-high",
		Provider: antigravity.ProviderID,
	}
	other.migrateSubscriptionModelSelections()
	assert.Equal(t, "gemini-3.8-flash-high", other.Models[SelectedModelTypeLarge].Model)

	// No subscription configured at all.
	none := &Config{Providers: csync.NewMap[string, ProviderConfig](), Models: map[SelectedModelType]SelectedModel{}}
	require.NotPanics(t, func() { none.migrateSubscriptionModelSelections() })
}
