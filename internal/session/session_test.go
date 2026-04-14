package session

import (
	"database/sql"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func TestMarshalModels(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		result, err := marshalModels(map[config.SelectedModelType]config.SelectedModel{})
		require.NoError(t, err)
		require.Equal(t, "", result)
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		result, err := marshalModels(nil)
		require.NoError(t, err)
		require.Equal(t, "", result)
	})

	t.Run("single entry", func(t *testing.T) {
		t.Parallel()
		models := map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				Model:    "claude-sonnet-4-20250514",
				Provider: "anthropic",
			},
		}
		result, err := marshalModels(models)
		require.NoError(t, err)
		require.Contains(t, result, "claude-sonnet-4-20250514")
		require.Contains(t, result, "anthropic")
	})

	t.Run("round-trip", func(t *testing.T) {
		t.Parallel()
		temp := 0.7
		topP := 0.9
		topK := int64(50)
		freqPen := 0.1
		presPen := 0.2
		models := map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				Model:            "gpt-4o",
				Provider:         "openai",
				ReasoningEffort:  "high",
				Think:            true,
				MaxTokens:        4096,
				Temperature:      &temp,
				TopP:             &topP,
				TopK:             &topK,
				FrequencyPenalty: &freqPen,
				PresencePenalty:  &presPen,
				ProviderOptions:  map[string]any{"key": "value"},
			},
			config.SelectedModelTypeSmall: {
				Model:    "gpt-4o-mini",
				Provider: "openai",
			},
		}
		data, err := marshalModels(models)
		require.NoError(t, err)
		result, err := unmarshalModels(data)
		require.NoError(t, err)
		require.Equal(t, models, result)
	})
}

func TestUnmarshalModels(t *testing.T) {
	t.Parallel()

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()
		result, err := unmarshalModels("")
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("valid JSON", func(t *testing.T) {
		t.Parallel()
		data := `{"large":{"model":"gpt-4o","provider":"openai"}}`
		result, err := unmarshalModels(data)
		require.NoError(t, err)
		require.Equal(t, "gpt-4o", result[config.SelectedModelTypeLarge].Model)
		require.Equal(t, "openai", result[config.SelectedModelTypeLarge].Provider)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, err := unmarshalModels("{invalid}")
		require.Error(t, err)
	})
}

func TestFromDBItemWithModels(t *testing.T) {
	t.Parallel()

	t.Run("null models", func(t *testing.T) {
		t.Parallel()
		item := testDBSession()
		item.Models = sql.NullString{Valid: false}
		result := service{}.fromDBItem(item)
		require.Nil(t, result.Models)
	})

	t.Run("empty models", func(t *testing.T) {
		t.Parallel()
		item := testDBSession()
		item.Models = sql.NullString{String: "", Valid: true}
		result := service{}.fromDBItem(item)
		require.Nil(t, result.Models)
	})

	t.Run("valid models", func(t *testing.T) {
		t.Parallel()
		item := testDBSession()
		item.Models = sql.NullString{
			String: `{"large":{"model":"gpt-4o","provider":"openai"}}`,
			Valid:  true,
		}
		result := service{}.fromDBItem(item)
		require.NotNil(t, result.Models)
		require.Equal(t, "gpt-4o", result.Models[config.SelectedModelTypeLarge].Model)
	})

	t.Run("invalid JSON models", func(t *testing.T) {
		t.Parallel()
		item := testDBSession()
		item.Models = sql.NullString{
			String: "{invalid}",
			Valid:  true,
		}
		result := service{}.fromDBItem(item)
		require.Nil(t, result.Models)
	})
}

func testDBSession() db.Session {
	return db.Session{
		ID:    "test-id",
		Title: "Test",
	}
}
