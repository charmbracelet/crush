package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestGetModelIncludesChatGPTModels(t *testing.T) {
	t.Parallel()

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("openai", ProviderConfig{
		Models:        []catwalk.Model{{ID: "gpt-5.1"}},
		ChatGPTModels: []catwalk.Model{{ID: "gpt-5.1-codex", Name: "GPT-5.1 Codex"}},
	})
	cfg := &Config{Providers: providers}

	model := cfg.GetModel("openai", "gpt-5.1-codex")
	require.NotNil(t, model)
	require.Equal(t, "GPT-5.1 Codex", model.Name)

	require.NotNil(t, cfg.GetModel("openai", "gpt-5.1"))
	require.Nil(t, cfg.GetModel("openai", "missing"))
}

func TestTokenFields(t *testing.T) {
	t.Parallel()

	t.Run("openai keeps the api key", func(t *testing.T) {
		t.Parallel()

		fields := tokenFields("openai", &oauth.Token{AccessToken: "at"})
		require.Contains(t, fields, "providers.openai.oauth")
		require.NotContains(t, fields, "providers.openai.api_key")
	})

	t.Run("other providers mirror the access token", func(t *testing.T) {
		t.Parallel()

		fields := tokenFields("hyper", &oauth.Token{AccessToken: "at"})
		require.Contains(t, fields, "providers.hyper.oauth")
		require.Equal(t, "at", fields["providers.hyper.api_key"])
	})
}

func TestTokenAccountIDRoundTrip(t *testing.T) {
	t.Parallel()

	// The account ID is persisted with the token in crush.json; verify
	// it survives the JSON round trip loadTokenFromDisk performs.
	token := &oauth.Token{
		AccessToken:  "at",
		RefreshToken: "rt",
		IDToken:      "idt",
		AccountID:    "acct-42",
		ExpiresIn:    3600,
		ExpiresAt:    4102444800,
	}

	data, err := json.Marshal(token)
	require.NoError(t, err)

	var restored oauth.Token
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Equal(t, "acct-42", restored.AccountID)
	require.Equal(t, "idt", restored.IDToken)
	require.Equal(t, "rt", restored.RefreshToken)
}

// testResolver mimics the shell resolver for templates: variables
// resolve to empty when unset, literals pass through.
type testResolver struct{}

func (testResolver) ResolveValue(v string) (string, error) {
	if strings.HasPrefix(v, "$") {
		return "", nil
	}
	return v, nil
}

func TestHasAPIKey(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		require.False(t, (&ProviderConfig{}).HasAPIKey(testResolver{}))
	})

	t.Run("literal key", func(t *testing.T) {
		t.Parallel()

		require.True(t, (&ProviderConfig{APIKey: "sk-key"}).HasAPIKey(testResolver{}))
	})

	t.Run("unresolved template", func(t *testing.T) {
		t.Parallel()

		require.False(t, (&ProviderConfig{APIKey: "$OPENAI_API_KEY"}).HasAPIKey(testResolver{}))
	})
}

// TestSetProviderAPIKeyOpenAIIsEitherOr proves the OpenAI provider holds
// exactly one credential: a ChatGPT login retires a previously entered
// API key, and entering an API key retires a previous ChatGPT login.
// Copilot keeps mirroring the access token into api_key as before.
func TestSetProviderAPIKeyOpenAIIsEitherOr(t *testing.T) {
	// Not parallel: t.Setenv below.

	// Point config discovery at the test sandbox: the write below
	// triggers an auto-reload, which must not pick up the developer's
	// real crush.json.
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	token := &oauth.Token{
		AccessToken:  "chatgpt-at",
		RefreshToken: "chatgpt-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	}

	newStore := func(t *testing.T, providerID string, initialConfig string) *ConfigStore {
		t.Helper()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "crush.json")
		require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0o600))

		// The in-memory config mirrors what a real load would produce.
		// SetConfigFields auto-reloads from configPath, so any state the
		// assertions expect to survive must be on disk.
		providers := csync.NewMap[string, ProviderConfig]()
		require.NoError(t, json.Unmarshal([]byte(initialConfig), &struct {
			Providers *csync.Map[string, ProviderConfig] `json:"providers"`
		}{Providers: providers}))

		return &ConfigStore{
			config:         &Config{Providers: providers},
			globalDataPath: configPath,
			workingDir:     dir,
			fetchOpenAIModels: func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
				return []catwalk.Model{{ID: "gpt-5.1-codex"}}, nil
			},
		}
	}

	t.Run("chatgpt login retires the api key", func(t *testing.T) {
		store := newStore(t, "openai", `{
			"providers": {
				"openai": {
					"id": "openai",
					"api_key": "sk-keep",
					"models": [{"id": "gpt-5.1", "name": "GPT-5.1"}]
				}
			}
		}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "openai", token))

		pc, ok := store.Config().Providers.Get("openai")
		require.True(t, ok)
		require.Empty(t, pc.APIKey, "the ChatGPT login replaces the API key")
		require.Equal(t, token, pc.OAuthToken)
		require.Equal(t, "gpt-5.1-codex", pc.ChatGPTModels[0].ID, "the subscription catalog lands in its own field")
		require.Equal(t, "gpt-5.1", pc.Models[0].ID, "the API catalog is untouched")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.NotContains(t, string(disk), "sk-keep", "the retired key is gone from the config file")
		require.Contains(t, string(disk), "chatgpt-rt", "the login is persisted")
	})

	t.Run("api key retires the chatgpt login", func(t *testing.T) {
		store := newStore(t, "openai", `{
			"providers": {
				"openai": {
					"id": "openai",
					"api_key": "",
					"oauth": {"access_token": "old-at", "refresh_token": "old-rt"},
					"chatgpt_models": [{"id": "gpt-5.6-luna"}]
				}
			}
		}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "openai", "sk-new"))

		pc, ok := store.Config().Providers.Get("openai")
		require.True(t, ok)
		require.Equal(t, "sk-new", pc.APIKey)
		require.Nil(t, pc.OAuthToken, "the API key replaces the ChatGPT login")
		require.Empty(t, pc.ChatGPTModels, "the subscription catalog goes with it")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.NotContains(t, string(disk), "old-rt", "the retired login is gone from the config file")
		require.NotContains(t, string(disk), "gpt-5.6-luna")
	})

	t.Run("copilot", func(t *testing.T) {
		store := newStore(t, "copilot", `{"providers":{"copilot":{"id":"copilot"}}}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "copilot", token))

		pc, ok := store.Config().Providers.Get("copilot")
		require.True(t, ok)
		require.Equal(t, "chatgpt-at", pc.APIKey, "copilot still mirrors the access token into api_key")
		require.Equal(t, token, pc.OAuthToken)
	})
}
