package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth/openai_codex"
	"github.com/stretchr/testify/require"
)

func TestEnsureOpenAICodexProvider(t *testing.T) {
	t.Parallel()

	providers := []catwalk.Provider{
		{
			ID: catwalk.InferenceProviderOpenAI,
			Models: []catwalk.Model{
				{ID: "gpt-5.4", Name: "gpt-5.4", ContextWindow: 272000, DefaultMaxTokens: 32768, CanReason: true},
				{ID: "gpt-5.4-mini", Name: "gpt-5.4-mini", ContextWindow: 272000, DefaultMaxTokens: 32768, CanReason: true},
			},
		},
	}

	got := ensureOpenAICodexProvider(providers)

	idx := -1
	for i, provider := range got {
		if provider.ID == catwalk.InferenceProvider(openai_codex.ProviderID) {
			idx = i
			break
		}
	}
	require.NotEqual(t, -1, idx)

	provider := got[idx]
	require.Equal(t, openAICodexBaseURL, provider.APIEndpoint)
	require.Equal(t, catwalk.TypeOpenAI, provider.Type)
	require.Equal(t, "gpt-5.4", provider.DefaultLargeModelID)
	require.Equal(t, "gpt-5.4-mini", provider.DefaultSmallModelID)
	require.GreaterOrEqual(t, len(provider.Models), 2)
}
