package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifierModelSelection(t *testing.T) {
	t.Parallel()

	explicit := &config.AutoModeConfig{
		Classifier: &config.AutoModeClassifier{
			Provider: "local-llama",
			Model:    "gemma4-e4b",
		},
	}

	t.Run("explicit classifier wins", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			AutoMode: explicit,
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeSmall: {Provider: "p", Model: "small-model"},
				config.SelectedModelTypeLarge: {Provider: "p", Model: "large-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "local-llama", sel.Provider)
		assert.Equal(t, "gemma4-e4b", sel.Model)
	})

	t.Run("falls back to small model", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeSmall: {Provider: "p", Model: "small-model"},
				config.SelectedModelTypeLarge: {Provider: "p", Model: "large-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "small-model", sel.Model)
	})

	t.Run("falls back to large model when small unset", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeLarge: {Provider: "p", Model: "large-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "large-model", sel.Model)
	})

	t.Run("nil auto mode section falls through to slots", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeSmall: {Provider: "p", Model: "small-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "small-model", sel.Model)
	})

	t.Run("empty classifier fields fall through to slots", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			AutoMode: &config.AutoModeConfig{
				Classifier: &config.AutoModeClassifier{Provider: "", Model: ""},
			},
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeLarge: {Provider: "p", Model: "large-model"},
			},
		}
		sel, ok := classifierModelSelection(cfg)
		require.True(t, ok)
		assert.Equal(t, "large-model", sel.Model)
	})

	t.Run("no models at all yields no selection", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		_, ok := classifierModelSelection(cfg)
		assert.False(t, ok, "rules-only mode applies when nothing is configured")
	})
}
