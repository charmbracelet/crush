package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

func TestConfig_ValidateReasoningEffort(t *testing.T) {
	t.Parallel()

	newConfig := func(models ...catwalk.Model) *Config {
		return &Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"openai": {
					ID:     "openai",
					Models: models,
				},
			}),
		}
	}

	t.Run("supported effort passes", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig(catwalk.Model{
			ID:              "gpt-5",
			CanReason:       true,
			ReasoningLevels: []string{"low", "medium", "high"},
		})
		require.NoError(t, cfg.ValidateReasoningEffort("openai", "gpt-5", "high"))
	})

	t.Run("unsupported effort lists accepted values", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig(catwalk.Model{
			ID:              "gpt-5",
			CanReason:       true,
			ReasoningLevels: []string{"low", "medium", "high"},
		})
		err := cfg.ValidateReasoningEffort("openai", "gpt-5", "ultra")
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not support reasoning effort")
		require.Contains(t, err.Error(), `"ultra"`)
		require.Contains(t, err.Error(), "accepted values: low, medium, high")
	})

	t.Run("model without reasoning levels rejects any effort", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig(catwalk.Model{ID: "gpt-4o"})
		err := cfg.ValidateReasoningEffort("openai", "gpt-4o", "low")
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not support reasoning effort")
	})

	t.Run("unknown model errors", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig(catwalk.Model{ID: "gpt-4o"})
		err := cfg.ValidateReasoningEffort("openai", "gpt-5", "low")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("empty effort is rejected like any other value", func(t *testing.T) {
		t.Parallel()
		cfg := newConfig(catwalk.Model{
			ID:              "gpt-5",
			CanReason:       true,
			ReasoningLevels: []string{"low", "medium", "high"},
		})
		require.Error(t, cfg.ValidateReasoningEffort("openai", "gpt-5", ""))
	})
}
