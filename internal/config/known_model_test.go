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

func TestConfig_IsKnownModelID_NoProviders(t *testing.T) {
	t.Parallel()

	cfg := newConfigWithProviders(t, nil)
	require.False(t, cfg.IsKnownModelID("gpt-4o"))
	require.False(t, cfg.IsKnownModelID(""))
}
