package agent

import (
	"encoding/json"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// Title generation never goes through getProviderOptions, so a model that
// needs provider-specific request fields (Qwen's chat_template_kwargs to
// disable thinking, for example) would lose them on the title call and
// burn the whole token budget reasoning. These tests pin that the title
// path forwards model provider_options.
func TestTitleProviderOptions(t *testing.T) {
	t.Run("empty options produce no provider options", func(t *testing.T) {
		m := Model{CatwalkCfg: catwalk.Model{ID: "qwen3.8-27b"}}
		require.Nil(t, titleProviderOptions(m))
	})

	t.Run("model provider_options are keyed for openai-compat transports", func(t *testing.T) {
		m := Model{
			CatwalkCfg: catwalk.Model{ID: "Qwen3.8-27B"},
			ModelCfg: config.SelectedModel{
				ProviderOptions: map[string]any{
					"extra_body": map[string]any{
						"chat_template_kwargs": map[string]any{"enable_thinking": false},
					},
				},
			},
		}

		opts := titleProviderOptions(m)
		require.Len(t, opts, 1)

		parsed, ok := opts[openaicompat.Name].(*openaicompat.ProviderOptions)
		require.True(t, ok, "options should parse as openai-compat provider options")
		require.NotNil(t, parsed.ExtraBody)

		body, err := json.Marshal(parsed.ExtraBody)
		require.NoError(t, err)

		var extra map[string]any
		require.NoError(t, json.Unmarshal(body, &extra))
		kwargs, ok := extra["chat_template_kwargs"].(map[string]any)
		require.True(t, ok, "chat_template_kwargs should survive the round trip: %v", extra)
		require.Equal(t, false, kwargs["enable_thinking"])
	})

	t.Run("options parse failures degrade to none", func(t *testing.T) {
		m := Model{
			ModelCfg: config.SelectedModel{
				ProviderOptions: map[string]any{
					"extra_body": make(chan int), // not JSON marshalable
				},
			},
		}
		require.Nil(t, titleProviderOptions(m))
	})

	t.Run("the generated options fit the fantasy call shape", func(t *testing.T) {
		m := Model{
			ModelCfg: config.SelectedModel{
				ProviderOptions: map[string]any{
					"extra_body": map[string]any{"k": "v"},
				},
			},
		}
		_ = titleProviderOptions(m)
	})
}
