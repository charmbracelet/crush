package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_ModelForProvider(t *testing.T) {
	t.Run("prefers the coder agent model", func(t *testing.T) {
		cfg := &Config{
			Models: map[SelectedModelType]SelectedModel{
				SelectedModelTypeLarge: {Provider: "openai", Model: "gpt-4o"},
				SelectedModelTypeSmall: {Provider: "anthropic", Model: "claude"},
			},
			Agents: map[string]Agent{
				AgentCoder: {Model: SelectedModelTypeSmall},
			},
		}

		model, modelType, ok := cfg.ModelForProvider("anthropic")
		assert.True(t, ok)
		assert.Equal(t, SelectedModelTypeSmall, modelType)
		assert.Equal(t, "claude", model.Model)
	})

	t.Run("falls back to the large model", func(t *testing.T) {
		cfg := &Config{
			Models: map[SelectedModelType]SelectedModel{
				SelectedModelTypeLarge: {Provider: "openai", Model: "gpt-4o"},
				SelectedModelTypeSmall: {Provider: "anthropic", Model: "claude"},
			},
		}

		model, modelType, ok := cfg.ModelForProvider("openai")
		assert.True(t, ok)
		assert.Equal(t, SelectedModelTypeLarge, modelType)
		assert.Equal(t, "gpt-4o", model.Model)
	})

	t.Run("falls back to the small model", func(t *testing.T) {
		cfg := &Config{
			Models: map[SelectedModelType]SelectedModel{
				SelectedModelTypeLarge: {Provider: "openai", Model: "gpt-4o"},
				SelectedModelTypeSmall: {Provider: "anthropic", Model: "claude"},
			},
		}

		model, modelType, ok := cfg.ModelForProvider("anthropic")
		assert.True(t, ok)
		assert.Equal(t, SelectedModelTypeSmall, modelType)
		assert.Equal(t, "claude", model.Model)
	})

	t.Run("reports no model for an unused provider", func(t *testing.T) {
		cfg := &Config{
			Models: map[SelectedModelType]SelectedModel{
				SelectedModelTypeLarge: {Provider: "openai", Model: "gpt-4o"},
			},
		}

		_, _, ok := cfg.ModelForProvider("kimi")
		assert.False(t, ok)
	})

	t.Run("reports no model when nothing is configured", func(t *testing.T) {
		cfg := &Config{}

		_, _, ok := cfg.ModelForProvider("openai")
		assert.False(t, ok)
	})
}
