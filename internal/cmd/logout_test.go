package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestLogoutCmd_Aliases(t *testing.T) {
	t.Parallel()

	require.Equal(t, "signout", logoutCmd.Aliases[0])
}

func TestLogoutCmd_HasForceFlag(t *testing.T) {
	t.Parallel()

	flag := logoutCmd.Flags().Lookup("force")
	require.NotNil(t, flag)
	require.Equal(t, "f", flag.Shorthand)
	require.Equal(t, "false", flag.DefValue)
}

func TestLogoutCmd_ValidArgs(t *testing.T) {
	t.Parallel()

	validPlatforms := map[string]bool{}
	for _, p := range logoutCmd.ValidArgs {
		validPlatforms[p] = true
	}
	require.True(t, validPlatforms["hyper"])
	require.True(t, validPlatforms["copilot"])
	require.True(t, validPlatforms["github"])
	require.True(t, validPlatforms["github-copilot"])
	require.True(t, validPlatforms["openai"])
	require.True(t, validPlatforms["chatgpt"])
	require.True(t, validPlatforms["codex"])
}

func TestLogoutContext_CreatesValidContext(t *testing.T) {
	ctx := getLogoutContext()
	require.NotNil(t, ctx)
	require.NoError(t, ctx.Err())
}

func TestLogoutOpenAI_RemovesOAuthAndPreservesAPIKey(t *testing.T) {
	t.Parallel()
	testLogoutOpenAI(t, true)
}

func TestLogoutOpenAI_RemovesOAuthWhenAPIKeyIsAbsent(t *testing.T) {
	t.Parallel()
	testLogoutOpenAI(t, false)
}

func testLogoutOpenAI(t *testing.T, apiKeyPresent bool) {
	t.Helper()
	removed := make(chan string, 1)
	provider := config.ProviderConfig{ID: string(catwalk.InferenceProviderOpenAI), OAuthToken: &oauth.Token{AccessToken: "oauth"}}
	if apiKeyPresent {
		provider.APIKey = "api-key"
	}
	cfg := config.Config{Providers: csync.NewMap[string, config.ProviderConfig]()}
	cfg.Providers.Set(string(catwalk.InferenceProviderOpenAI), provider)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/workspaces/ws/config":
			_ = json.NewEncoder(w).Encode(cfg)
		case "/v1/workspaces/ws/config/remove":
			var request struct {
				Key string `json:"key"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			removed <- request.Key
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c, err := client.NewClient(t.TempDir(), "tcp", server.Listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, logoutOpenAI(c, "ws"))
	require.Equal(t, "providers.openai.oauth", <-removed)
	require.Equal(t, map[bool]string{
		true:  "Removed ChatGPT/Codex OAuth credentials. Your OpenAI API key was preserved.",
		false: "Successfully logged out of ChatGPT/Codex.",
	}[apiKeyPresent], openAILogoutMessage(apiKeyPresent))
}
