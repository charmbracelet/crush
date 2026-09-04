package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIsOpenCodeMessagesModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider string
		model    string
		expected bool
	}{
		{"opencode-zen", "qwen3.7-max", true},
		{"opencode-zen", "qwen3.5-plus", true},
		{"opencode-zen", "qwen3.6-plus", true},
		{"opencode-zen", "qwen3.7-plus", true},
		{"opencode-zen", "claude-opus-4-5", true},
		{"opencode-zen", "claude-sonnet-5", true},
		{"opencode-zen", "muse-spark-1.3-contributor-free", false},
		{"opencode-zen", "gpt-5.6-luna", false},
		{"opencode-zen", "grok-4.5", false},
		{"opencode-zen", "minimax-m3", false},
		{"opencode-zen", "kimi-k3", false},
		{"opencode-zen", "big-pickle", false},
		{"opencode-go", "minimax-m2.7", true},
		{"opencode-go", "minimax-m3", true},
		{"opencode-go", "qwen3.7-max", true},
		{"opencode-go", "qwen3.7-plus", true},
		{"opencode-go", "qwen3.6-plus", true},
		{"opencode-go", "qwen3.8-flash", true},
		{"opencode-go", "qwen3.8-max", true},
		{"opencode-go", "muse-spark-1.3-contributor", false},
		{"opencode-go", "gpt-5.6-luna", false},
		{"opencode-go", "glm-5.3", false},
		{"opencode-go", "kimi-k3", false},
		{"opencode-go", "longcat-2.0", false},
		{"opencode-go", "ox-alpha-free", false},
		{"opencode-go", "minimax", false},
		{"other", "claude-opus-4-5", false},
		{"other", "qwen3.7-max", false},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, isOpenCodeMessagesModel(tt.provider, tt.model), "%s/%s", tt.provider, tt.model)
	}
}

func TestIsOpenCodeResponsesModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model    string
		expected bool
	}{
		{"muse-spark-1.3-contributor-free", true},
		{"muse-spark-1.2", true},
		{"grok-4.5", true},
		{"grok-4.6", true},
		{"grok-build-0.1", true},
		{"gpt-5.6-luna", true},
		{"gpt-5.5", true},
		{"gpt-5.3-codex", true},
		{"minimax-m3", false},
		{"qwen3.7-max", false},
		{"kimi-k3", false},
		{"glm-5.3", false},
		{"big-pickle", false},
		{"ox-alpha-free", false},
		{"hy3", false},
		{"longcat-2.0", false},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, isOpenCodeResponsesModel(tt.model), tt.model)
	}
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
				Type:    catwalk.TypeOpenAICompat,
				APIKey:  "$OPENCODE_API_KEY",
			}
			coord := newTestCoordinator(t, env, providerID, providerCfg)

			for _, modelID := range []string{"kimi-k3", "muse-spark-1.3-contributor-free", "grok-4.6", "gpt-5.6-luna"} {
				provider, err := coord.buildProvider(providerCfg, config.SelectedModel{
					Model:    modelID,
					Provider: providerID,
				}, false)
				require.NoError(t, err, modelID)
				require.NotNil(t, provider, modelID)
			}
		})
	}
}
