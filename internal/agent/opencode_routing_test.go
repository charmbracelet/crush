package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestServingProvider(t *testing.T) {
	t.Parallel()

	providerCfg := func(id string, providerType catwalk.Type) config.ProviderConfig {
		return config.ProviderConfig{ID: id, Type: providerType}
	}
	model := func(modelType catwalk.Type) catwalk.Model {
		return catwalk.Model{ID: "some-model", Type: modelType}
	}

	t.Run("bespoke SDKs are selected by provider ID", func(t *testing.T) {
		t.Parallel()
		for providerID, expected := range map[string]string{
			string(catwalk.InferenceProviderBedrock):       "bedrock",
			string(catwalk.InferenceProviderBedrockEurope): "bedrock",
			string(catwalk.InferenceProviderAzure):         "azure",
			string(catwalk.InferenceProviderGemini):        "google",
			string(catwalk.InferenceProviderVertexAI):      "google",
			string(catwalk.InferenceProviderOpenRouter):    "openrouter",
			string(catwalk.InferenceProviderVercel):        "vercel",
			"hyper":                                        "hyper",
			string(catwalk.InferenceProviderCopilot):       "openai-compat",
			string(catwalk.InferenceProviderOpenCodeGo):    "openai-compat",
			string(catwalk.InferenceProviderOpenCodeZen):   "openai-compat",
		} {
			require.Equal(t, expected, servingProvider(providerCfg(providerID, catwalk.TypeCompletions), model("")), providerID)
		}
	})

	t.Run("generic providers are selected by the model's wire type", func(t *testing.T) {
		t.Parallel()
		for wireType, expected := range map[catwalk.Type]string{
			catwalk.TypeResponses:   "openai",
			catwalk.TypeMessages:    "anthropic",
			catwalk.TypeCompletions: "openai-compat",
		} {
			require.Equal(t, expected, servingProvider(providerCfg("custom", ""), model(wireType)), wireType)
			require.Equal(t, expected, servingProvider(providerCfg("custom", wireType), model("")), wireType)
		}
	})

	t.Run("custom provider types are passed through", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "litellm", servingProvider(providerCfg("litellm", "litellm"), model("")))
	})
}

func TestBuildProviderOpenCodeRouting(t *testing.T) {
	t.Parallel()

	for _, providerID := range []string{
		string(catwalk.InferenceProviderOpenCodeZen),
		string(catwalk.InferenceProviderOpenCodeGo),
	} {
		t.Run(providerID, func(t *testing.T) {
			t.Parallel()
			env := testEnv(t)
			providerCfg := config.ProviderConfig{
				ID:      providerID,
				BaseURL: "https://opencode.ai/zen/v1",
				Type:    catwalk.TypeCompletions,
				APIKey:  "$OPENCODE_API_KEY",
			}
			coord := newTestCoordinator(t, env, providerID, providerCfg)

			for _, wireType := range []catwalk.Type{catwalk.TypeCompletions, catwalk.TypeResponses, catwalk.TypeMessages} {
				provider, err := coord.buildProvider(providerCfg, config.SelectedModel{
					Model:    "some-model",
					Provider: providerID,
				}, false, wireType)
				require.NoError(t, err, wireType)
				require.NotNil(t, provider, wireType)
			}
		})
	}
}
