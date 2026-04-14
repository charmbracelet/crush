package proto

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestModelsRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("nil models", func(t *testing.T) {
		t.Parallel()
		s := Session{
			ID:    "test-id",
			Title: "Test",
		}
		data, err := json.Marshal(s)
		require.NoError(t, err)
		var decoded Session
		require.NoError(t, json.Unmarshal(data, &decoded))
		require.Nil(t, decoded.Models)
	})

	t.Run("populated models", func(t *testing.T) {
		t.Parallel()
		temp := 0.7
		s := Session{
			ID:    "test-id",
			Title: "Test",
			Models: map[SelectedModelType]SelectedModel{
				SelectedModelTypeLarge: {
					Model:           "gpt-4o",
					Provider:        "openai",
					ReasoningEffort: "high",
					Think:           true,
					MaxTokens:       4096,
					Temperature:     &temp,
					ProviderOptions: map[string]any{"key": "value"},
				},
				SelectedModelTypeSmall: {
					Model:    "gpt-4o-mini",
					Provider: "openai",
				},
			},
		}
		data, err := json.Marshal(s)
		require.NoError(t, err)
		var decoded Session
		require.NoError(t, json.Unmarshal(data, &decoded))
		require.Equal(t, "gpt-4o", decoded.Models[SelectedModelTypeLarge].Model)
		require.Equal(t, "openai", decoded.Models[SelectedModelTypeLarge].Provider)
		require.Equal(t, "high", decoded.Models[SelectedModelTypeLarge].ReasoningEffort)
		require.True(t, decoded.Models[SelectedModelTypeLarge].Think)
		require.Equal(t, int64(4096), decoded.Models[SelectedModelTypeLarge].MaxTokens)
		require.NotNil(t, decoded.Models[SelectedModelTypeLarge].Temperature)
		require.Equal(t, 0.7, *decoded.Models[SelectedModelTypeLarge].Temperature)
		require.Equal(t, "gpt-4o-mini", decoded.Models[SelectedModelTypeSmall].Model)
	})

	t.Run("empty map models", func(t *testing.T) {
		t.Parallel()
		s := Session{
			ID:     "test-id",
			Title:  "Test",
			Models: map[SelectedModelType]SelectedModel{},
		}
		data, err := json.Marshal(s)
		require.NoError(t, err)
		var decoded Session
		require.NoError(t, json.Unmarshal(data, &decoded))
		// Empty map with omitempty is dropped during marshaling.
		require.Nil(t, decoded.Models)
	})
}

func TestProtoToDomainRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("models through proto", func(t *testing.T) {
		t.Parallel()
		temp := 0.7
		domainModels := map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				Model:           "gpt-4o",
				Provider:        "openai",
				ReasoningEffort: "high",
				Think:           true,
				MaxTokens:       4096,
				Temperature:     &temp,
				ProviderOptions: map[string]any{"key": "value"},
			},
		}

		// Domain → Proto
		protoModels := convertModelsToProtoLocal(domainModels)
		require.Equal(t, SelectedModelTypeLarge, SelectedModelType(config.SelectedModelTypeLarge))
		require.Equal(t, "gpt-4o", protoModels[SelectedModelTypeLarge].Model)
		require.Equal(t, "openai", protoModels[SelectedModelTypeLarge].Provider)

		// Proto → Domain
		result := convertModelsFromProtoLocal(protoModels)
		require.Equal(t, "gpt-4o", result[config.SelectedModelTypeLarge].Model)
		require.Equal(t, "openai", result[config.SelectedModelTypeLarge].Provider)
		require.Equal(t, "high", result[config.SelectedModelTypeLarge].ReasoningEffort)
		require.True(t, result[config.SelectedModelTypeLarge].Think)
		require.Equal(t, int64(4096), result[config.SelectedModelTypeLarge].MaxTokens)
		require.NotNil(t, result[config.SelectedModelTypeLarge].Temperature)
		require.Equal(t, 0.7, *result[config.SelectedModelTypeLarge].Temperature)
	})

	t.Run("nil models round-trip", func(t *testing.T) {
		t.Parallel()
		protoModels := convertModelsToProtoLocal(nil)
		require.Nil(t, protoModels)

		domainModels := convertModelsFromProtoLocal(nil)
		require.Nil(t, domainModels)
	})
}

func convertModelsToProtoLocal(models map[config.SelectedModelType]config.SelectedModel) map[SelectedModelType]SelectedModel {
	if models == nil {
		return nil
	}
	result := make(map[SelectedModelType]SelectedModel, len(models))
	for k, v := range models {
		result[SelectedModelType(k)] = SelectedModel{
			Model:            v.Model,
			Provider:         v.Provider,
			ReasoningEffort:  v.ReasoningEffort,
			Think:            v.Think,
			MaxTokens:        v.MaxTokens,
			Temperature:      v.Temperature,
			TopP:             v.TopP,
			TopK:             v.TopK,
			FrequencyPenalty: v.FrequencyPenalty,
			PresencePenalty:  v.PresencePenalty,
			ProviderOptions:  v.ProviderOptions,
		}
	}
	return result
}

func convertModelsFromProtoLocal(models map[SelectedModelType]SelectedModel) map[config.SelectedModelType]config.SelectedModel {
	if models == nil {
		return nil
	}
	result := make(map[config.SelectedModelType]config.SelectedModel, len(models))
	for k, v := range models {
		result[config.SelectedModelType(k)] = config.SelectedModel{
			Model:            v.Model,
			Provider:         v.Provider,
			ReasoningEffort:  v.ReasoningEffort,
			Think:            v.Think,
			MaxTokens:        v.MaxTokens,
			Temperature:      v.Temperature,
			TopP:             v.TopP,
			TopK:             v.TopK,
			FrequencyPenalty: v.FrequencyPenalty,
			PresencePenalty:  v.PresencePenalty,
			ProviderOptions:  v.ProviderOptions,
		}
	}
	return result
}
