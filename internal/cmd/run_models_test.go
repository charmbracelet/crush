package cmd

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestFindModelMatchesUsesCredentialScopedCatalog(t *testing.T) {
	t.Parallel()

	providers := map[string]config.ProviderConfig{
		"copilot": {
			ID:            "copilot",
			OAuthToken:    &oauth.Token{AccessToken: "token"},
			Models:        []catwalk.Model{{ID: "static-model"}},
			CopilotModels: []catwalk.Model{{ID: "auto"}},
		},
	}

	auto, _ := findModelMatches(providers, "copilot/auto", "")
	require.Equal(t, []modelMatch{{provider: "copilot", modelID: "auto"}}, auto)

	static, _ := findModelMatches(providers, "copilot/static-model", "")
	require.Empty(t, static)
}
