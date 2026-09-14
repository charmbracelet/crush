package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// TestSetProviderAPIKeyXAIIsEitherOr proves the xAI provider holds exactly
// one credential: a Grok login replaces a previously entered API key (the
// access token mirrors into it), and entering an API key retires a
// previous Grok login so its refreshes cannot shadow the key.
func TestSetProviderAPIKeyXAIIsEitherOr(t *testing.T) {
	// Not parallel: t.Setenv below.

	// Point config discovery at the test sandbox: the write below
	// triggers an auto-reload, which must not pick up the developer's
	// real crush.json.
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	token := &oauth.Token{
		AccessToken:  "grok-at",
		RefreshToken: "grok-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	}

	newStore := func(t *testing.T, initialConfig string) *ConfigStore {
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
		}
	}

	t.Run("grok login replaces the api key", func(t *testing.T) {
		store := newStore(t, `{
			"providers": {
				"xai": {
					"id": "xai",
					"api_key": "sk-keep",
					"models": [{"id": "grok-4.5", "name": "Grok 4.5"}]
				}
			}
		}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "xai", token))

		pc, ok := store.Config().Providers.Get("xai")
		require.True(t, ok)
		// The access token mirrors into api_key for non-OpenAI
		// providers: it is the credential the API client sends.
		require.Equal(t, token.AccessToken, pc.APIKey)
		require.Equal(t, token, pc.OAuthToken)
		require.Equal(t, "grok-4.5", pc.Models[0].ID, "the catalog is untouched")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.NotContains(t, string(disk), "sk-keep", "the retired key is gone from the config file")
		require.Contains(t, string(disk), "grok-rt", "the login is persisted")
	})

	t.Run("api key retires the grok login", func(t *testing.T) {
		store := newStore(t, `{
			"providers": {
				"xai": {
					"id": "xai",
					"api_key": "",
					"oauth": {"access_token": "old-at", "refresh_token": "old-rt"}
				}
			}
		}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "xai", "sk-new"))

		pc, ok := store.Config().Providers.Get("xai")
		require.True(t, ok)
		require.Equal(t, "sk-new", pc.APIKey)
		require.Nil(t, pc.OAuthToken, "the API key retires the Grok login")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.NotContains(t, string(disk), "old-rt", "the retired login is gone from the config file")
		require.Contains(t, string(disk), "sk-new")
	})
}
