package agent

import (
	"encoding/json"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/discover"
	"github.com/stretchr/testify/require"
)

// Title generation reuses getProviderOptions (per PR #3717 review), so
// these tests pin the options the title call forwards for a model that
// needs provider-specific request fields (Qwen's chat_template_kwargs to
// disable thinking, for example).
func TestTitleProviderOptions(t *testing.T) {
	qwenModel := Model{
		CatwalkCfg: catwalk.Model{ID: "qwen3-32b"},
		ModelCfg: config.SelectedModel{
			Provider: "qwen",
			ProviderOptions: map[string]any{
				"extra_body": map[string]any{
					"chat_template_kwargs": map[string]any{"enable_thinking": false},
				},
			},
		},
	}

	t.Run("a model without provider_options gets no chat_template_kwargs", func(t *testing.T) {
		m := Model{CatwalkCfg: catwalk.Model{ID: "qwen3-32b"}}
		opts := getProviderOptions(m, config.ProviderConfig{Type: catwalk.TypeOpenAICompat})
		parsed, ok := opts[openaicompat.Name].(*openaicompat.ProviderOptions)
		require.True(t, ok)
		if parsed.ExtraBody == nil {
			return
		}
		body, err := json.Marshal(parsed.ExtraBody)
		require.NoError(t, err)
		require.NotContains(t, string(body), "chat_template_kwargs")
	})

	t.Run("model provider_options survive for custom openai-compat providers", func(t *testing.T) {
		require.NotEmpty(t, discover.RegisteredProviderTypes(), "expected registered custom provider types")

		providerCfg := config.ProviderConfig{
			ID:   "qwen",
			Type: catwalk.Type(discover.RegisteredProviderTypes()[0]),
		}

		opts := getProviderOptions(qwenModel, providerCfg)
		parsed, ok := opts[openaicompat.Name].(*openaicompat.ProviderOptions)
		require.True(t, ok, "options should parse as openai-compat provider options")

		body, err := json.Marshal(parsed.ExtraBody)
		require.NoError(t, err)

		var extra map[string]any
		require.NoError(t, json.Unmarshal(body, &extra))
		kwargs, ok := extra["chat_template_kwargs"].(map[string]any)
		require.True(t, ok, "chat_template_kwargs should survive: %v", extra)
		require.Equal(t, false, kwargs["enable_thinking"])
	})
}
