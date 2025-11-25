package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToggleFavoriteModel(t *testing.T) {
	t.Parallel()

	// Define some models to use in tests
	largeModel1 := SelectedModel{Provider: "openai", Model: "gpt-4"}
	largeModel2 := SelectedModel{Provider: "anthropic", Model: "claude-3-opus"}
	smallModel1 := SelectedModel{Provider: "groq", Model: "llama3-8b"}

	// Helper function to create a new config with a temp file for testing
	newTestConfig := func(t *testing.T) *Config {
		t.Helper()
		tempDir := t.TempDir()
		tempConfigPath := filepath.Join(tempDir, "crush.json")
		// Create an empty config file to ensure ReadFile doesn't fail
		err := os.WriteFile(tempConfigPath, []byte("{}"), 0600)
		require.NoError(t, err)

		return &Config{
			dataConfigDir:   tempConfigPath,
			FavoritedModels: make(map[SelectedModelType][]SelectedModel),
		}
	}

	t.Run("add first favorite to large models", func(t *testing.T) {
		t.Parallel()
		cfg := newTestConfig(t)

		err := cfg.ToggleFavoriteModel(SelectedModelTypeLarge, largeModel1)
		require.NoError(t, err)

		require.Len(t, cfg.FavoritedModels, 1, "FavoritedModels map should have one entry for large models")
		require.Len(t, cfg.FavoritedModels[SelectedModelTypeLarge], 1, "Large model favorites list should have one model")
		require.Equal(t, largeModel1, cfg.FavoritedModels[SelectedModelTypeLarge][0], "The correct model should be in the large list")
		require.Empty(t, cfg.FavoritedModels[SelectedModelTypeSmall], "Small model favorites list should be empty")
	})

	t.Run("add a second favorite to large models", func(t *testing.T) {
		t.Parallel()
		cfg := newTestConfig(t)
		cfg.FavoritedModels[SelectedModelTypeLarge] = []SelectedModel{largeModel1}

		err := cfg.ToggleFavoriteModel(SelectedModelTypeLarge, largeModel2)
		require.NoError(t, err)

		require.Len(t, cfg.FavoritedModels[SelectedModelTypeLarge], 2, "Large model favorites list should have two models")
		require.Contains(t, cfg.FavoritedModels[SelectedModelTypeLarge], largeModel1)
		require.Contains(t, cfg.FavoritedModels[SelectedModelTypeLarge], largeModel2)
	})

	t.Run("remove a favorite from large models", func(t *testing.T) {
		t.Parallel()
		cfg := newTestConfig(t)
		cfg.FavoritedModels[SelectedModelTypeLarge] = []SelectedModel{largeModel2, largeModel1}

		// Toggle the first model to remove it
		err := cfg.ToggleFavoriteModel(SelectedModelTypeLarge, largeModel1)
		require.NoError(t, err)

		require.Len(t, cfg.FavoritedModels[SelectedModelTypeLarge], 1, "Large model favorites list should have one model remaining")
		require.Equal(t, largeModel2, cfg.FavoritedModels[SelectedModelTypeLarge][0], "The correct model should be remaining in the list")
	})

	t.Run("ensure large and small lists are independent", func(t *testing.T) {
		t.Parallel()
		cfg := newTestConfig(t)

		// Add one to large, one to small
		err := cfg.ToggleFavoriteModel(SelectedModelTypeLarge, largeModel1)
		require.NoError(t, err)
		err = cfg.ToggleFavoriteModel(SelectedModelTypeSmall, smallModel1)
		require.NoError(t, err)

		// Assert initial state
		require.Len(t, cfg.FavoritedModels[SelectedModelTypeLarge], 1, "Large list should have one model")
		require.Equal(t, largeModel1, cfg.FavoritedModels[SelectedModelTypeLarge][0])
		require.Len(t, cfg.FavoritedModels[SelectedModelTypeSmall], 1, "Small list should have one model")
		require.Equal(t, smallModel1, cfg.FavoritedModels[SelectedModelTypeSmall][0])

		// Remove from large list
		err = cfg.ToggleFavoriteModel(SelectedModelTypeLarge, largeModel1)
		require.NoError(t, err)

		// Assert final state
		require.Empty(t, cfg.FavoritedModels[SelectedModelTypeLarge], "Large list should be empty")
		require.Len(t, cfg.FavoritedModels[SelectedModelTypeSmall], 1, "Small list should be unchanged")
		require.Equal(t, smallModel1, cfg.FavoritedModels[SelectedModelTypeSmall][0])
	})

	t.Run("toggling the same model twice should result in an empty list", func(t *testing.T) {
		t.Parallel()
		cfg := newTestConfig(t)

		// Add it
		err := cfg.ToggleFavoriteModel(SelectedModelTypeLarge, largeModel1)
		require.NoError(t, err)
		require.Len(t, cfg.FavoritedModels[SelectedModelTypeLarge], 1)

		// Remove it
		err = cfg.ToggleFavoriteModel(SelectedModelTypeLarge, largeModel1)
		require.NoError(t, err)
		require.Empty(t, cfg.FavoritedModels[SelectedModelTypeLarge], "Toggling twice should remove the model, leaving an empty list")
	})
}
