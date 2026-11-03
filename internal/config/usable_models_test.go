package config

import (
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func modelsOf(ids ...string) []catwalk.Model {
	models := make([]catwalk.Model, 0, len(ids))
	for _, id := range ids {
		models = append(models, catwalk.Model{ID: id})
	}
	return models
}

func TestProviderConfig_SubscriptionModels(t *testing.T) {
	t.Parallel()

	token := &oauth.Token{AccessToken: "t"}
	tests := []struct {
		name string
		pc   ProviderConfig
		want []string
	}{
		{
			name: "chatgpt_login_with_a_fetched_list",
			pc:   ProviderConfig{ID: string(catwalk.InferenceProviderOpenAI), OAuthToken: token, ChatGPTModels: modelsOf("gpt-5")},
			want: []string{"gpt-5"},
		},
		{
			name: "grok_login_with_a_fetched_list",
			pc:   ProviderConfig{ID: string(catwalk.InferenceProviderXAI), OAuthToken: token, GrokModels: modelsOf("grok-4")},
			want: []string{"grok-4"},
		},
		{
			name: "api_key_has_no_subscription",
			pc:   ProviderConfig{ID: string(catwalk.InferenceProviderOpenAI), ChatGPTModels: modelsOf("gpt-5")},
		},
		{
			name: "login_before_the_list_is_fetched",
			pc:   ProviderConfig{ID: string(catwalk.InferenceProviderOpenAI), OAuthToken: token},
		},
		{
			name: "other_providers_have_no_subscription",
			pc:   ProviderConfig{ID: "copilot", OAuthToken: token, ChatGPTModels: modelsOf("ignored")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.pc.SubscriptionModels()
			if tt.want == nil {
				require.Empty(t, got)
				return
			}
			require.Equal(t, tt.want, modelIDs(got))
		})
	}
}

func TestProviderConfig_UsableModels(t *testing.T) {
	t.Parallel()

	token := &oauth.Token{AccessToken: "t"}
	tests := []struct {
		name string
		pc   ProviderConfig
		want []string
	}{
		{
			name: "api_key_lists_the_catalog",
			pc:   ProviderConfig{ID: string(catwalk.InferenceProviderOpenAI), Models: modelsOf("gpt-5", "gpt-5-mini")},
			want: []string{"gpt-5", "gpt-5-mini"},
		},
		{
			name: "chatgpt_login_lists_only_the_subscription_models",
			pc: ProviderConfig{
				ID:            string(catwalk.InferenceProviderOpenAI),
				OAuthToken:    token,
				Models:        modelsOf("gpt-5", "gpt-5-mini"),
				ChatGPTModels: modelsOf("gpt-5"),
			},
			want: []string{"gpt-5"},
		},
		{
			name: "grok_login_lists_only_the_subscription_models",
			pc: ProviderConfig{
				ID:         string(catwalk.InferenceProviderXAI),
				OAuthToken: token,
				Models:     modelsOf("grok-4", "grok-3"),
				GrokModels: modelsOf("grok-4"),
			},
			want: []string{"grok-4"},
		},
		{
			name: "login_without_a_fetched_subscription_list_falls_back_to_the_catalog",
			pc: ProviderConfig{
				ID:         string(catwalk.InferenceProviderOpenAI),
				OAuthToken: token,
				Models:     modelsOf("gpt-5"),
			},
			want: []string{"gpt-5"},
		},
		{
			name: "other_oauth_providers_list_the_catalog",
			pc: ProviderConfig{
				ID:            "copilot",
				OAuthToken:    token,
				Models:        modelsOf("gpt-4.1"),
				ChatGPTModels: modelsOf("ignored"),
			},
			want: []string{"gpt-4.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, modelIDs(tt.pc.UsableModels()))
		})
	}
}

// TestConfig_FindModelProvider_SkipsModelsALoginCannotUse pins that a
// subagent dispatch cannot pick an API-catalog model the signed-in
// subscription does not grant: it would only 404 at the provider.
func TestConfig_FindModelProvider_SkipsModelsALoginCannotUse(t *testing.T) {
	t.Parallel()

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set(string(catwalk.InferenceProviderOpenAI), ProviderConfig{
		ID:            string(catwalk.InferenceProviderOpenAI),
		OAuthToken:    &oauth.Token{AccessToken: "t"},
		Models:        modelsOf("gpt-5", "gpt-5-mini"),
		ChatGPTModels: modelsOf("gpt-5"),
	})
	cfg := &Config{Providers: providers}

	for _, provider := range []string{"", string(catwalk.InferenceProviderOpenAI)} {
		_, m, err := cfg.FindModelProvider(provider, "gpt-5")
		require.NoError(t, err)
		require.Equal(t, "gpt-5", m.ID)

		_, _, err = cfg.FindModelProvider(provider, "gpt-5-mini")
		require.ErrorContains(t, err, "gpt-5-mini")
		require.NoError(t, cfg.ValidateModel(provider, "gpt-5"))
		require.Error(t, cfg.ValidateModel(provider, "gpt-5-mini"))
	}
}

func TestConfig_FindModelProvider_HintOmitsModelsALoginCannotUse(t *testing.T) {
	t.Parallel()

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set(string(catwalk.InferenceProviderOpenAI), ProviderConfig{
		ID:            string(catwalk.InferenceProviderOpenAI),
		OAuthToken:    &oauth.Token{AccessToken: "t"},
		Models:        modelsOf("gpt-5", "gpt-5-mini"),
		ChatGPTModels: modelsOf("gpt-5"),
	})
	cfg := &Config{Providers: providers}

	_, _, err := cfg.FindModelProvider("", "gpt")
	require.ErrorContains(t, err, "similar: gpt-5")
	require.NotContains(t, err.Error(), "gpt-5-mini")
}
