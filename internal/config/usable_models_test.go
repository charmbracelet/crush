package config

import (
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestProviderConfigUsableModels(t *testing.T) {
	t.Parallel()
	api := []catwalk.Model{{ID: "api-model"}}
	sub := []catwalk.Model{{ID: "sub-model"}}
	token := &oauth.Token{AccessToken: "t"}

	tests := []struct {
		name string
		p    ProviderConfig
		want []catwalk.Model
	}{
		{"api key lists the catalog", ProviderConfig{ID: "openai", Models: api, SubscriptionCatalog: sub}, api},
		{"chatgpt login lists the subscription", ProviderConfig{ID: "openai", Models: api, SubscriptionCatalog: sub, OAuthToken: token}, sub},
		{"grok login lists the subscription", ProviderConfig{ID: "xai", Models: api, SubscriptionCatalog: sub, OAuthToken: token}, sub},
		{"an unfetched subscription falls back to the catalog", ProviderConfig{ID: "copilot", Models: api, OAuthToken: token}, api},
		{"other oauth providers keep the catalog", ProviderConfig{ID: "hyper", Models: api, SubscriptionCatalog: sub, OAuthToken: token}, api},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.p.UsableModels())
		})
	}
}

func TestProviderConfigSubscriptionModels(t *testing.T) {
	t.Parallel()
	token := &oauth.Token{AccessToken: "t"}
	_, ok := ProviderConfig{ID: "openai"}.SubscriptionModels()
	require.False(t, ok, "an API key is not a subscription")
	models, ok := ProviderConfig{ID: "openai", OAuthToken: token}.SubscriptionModels()
	require.True(t, ok)
	require.Nil(t, models, "signed in but the list is not fetched yet")
	_, ok = ProviderConfig{ID: "hyper", OAuthToken: token}.SubscriptionModels()
	require.False(t, ok, "hyper logins keep the regular catalog")
}
