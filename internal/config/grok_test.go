package config

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
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

		originalFetch := fetchGrokModels
		fetchGrokModels = func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
			return []catwalk.Model{{ID: "grok-4-fast"}}, nil
		}
		t.Cleanup(func() { fetchGrokModels = originalFetch })

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
		require.Equal(t, "grok-4-fast", pc.GrokModels[0].ID, "the subscription catalog lands in its own field")
		require.Equal(t, "grok-4.5", pc.Models[0].ID, "the API catalog is untouched")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.NotContains(t, string(disk), "sk-keep", "the retired key is gone from the config file")
		require.Contains(t, string(disk), "grok-rt", "the login is persisted")
		require.Contains(t, string(disk), "grok-4-fast", "the fetched catalog is persisted")
	})

	t.Run("api key retires the grok login", func(t *testing.T) {
		store := newStore(t, `{
			"providers": {
				"xai": {
					"id": "xai",
					"api_key": "",
					"oauth": {"access_token": "old-at", "refresh_token": "old-rt"},
					"grok_models": [{"id": "grok-4.20"}]
				}
			}
		}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "xai", "sk-new"))

		pc, ok := store.Config().Providers.Get("xai")
		require.True(t, ok)
		require.Equal(t, "sk-new", pc.APIKey)
		require.Nil(t, pc.OAuthToken, "the API key retires the Grok login")
		require.Empty(t, pc.GrokModels, "the subscription catalog goes with it")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.NotContains(t, string(disk), "old-rt", "the retired login is gone from the config file")
		require.NotContains(t, string(disk), "grok-4.20", "the retired catalog is gone from the config file")
		require.Contains(t, string(disk), "sk-new")
	})
}

// TestRefetchGrokModelsRefreshesExpiredToken pins the behavior of the
// catalog refresh when the stored Grok token has expired: the refresh
// renews the token before fetching, so the request is not rejected with
// a 401, and the fetched catalog replaces the persisted one.
func TestRefetchGrokModelsRefreshesExpiredToken(t *testing.T) {
	// Not parallel: swaps the fetchGrokModels package variable.
	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	expired := &oauth.Token{
		AccessToken:  "expired-at",
		RefreshToken: "expired-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
	}
	fresh := &oauth.Token{
		AccessToken:  "fresh-at",
		RefreshToken: "fresh-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	}

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("xai", ProviderConfig{
		ID:         "xai",
		OAuthToken: expired,
		GrokModels: []catwalk.Model{{ID: "grok-stale"}},
	})
	store := &ConfigStore{
		config:         &Config{Providers: providers},
		globalDataPath: configPath,
		exchangeToken: func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error) {
			require.Equal(t, "xai", providerID)
			require.Equal(t, "expired-rt", refreshToken)
			return fresh, nil
		},
	}

	originalFetch := fetchGrokModels
	var fetchToken *oauth.Token
	fetchGrokModels = func(_ context.Context, token *oauth.Token) ([]catwalk.Model, error) {
		fetchToken = token
		return []catwalk.Model{{ID: "grok-fresh"}}, nil
	}
	t.Cleanup(func() { fetchGrokModels = originalFetch })

	store.refetchGrokModels(t.Context(), ScopeGlobal)

	require.Equal(t, fresh, fetchToken, "the catalog is fetched with the renewed token")

	pc, ok := store.Config().Providers.Get("xai")
	require.True(t, ok)
	require.Equal(t, fresh, pc.OAuthToken, "the renewed token is kept in the config")
	require.Equal(t, "grok-fresh", pc.GrokModels[0].ID, "the existing catalog is replaced")

	disk, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(disk), "grok-fresh", "the fetched catalog is persisted")
	require.Contains(t, string(disk), "fresh-rt", "the renewed token is persisted")
}

// grokLoginConfig is a project config for a Grok login whose persisted
// model catalog has gone stale.
func grokLoginConfig(token *oauth.Token) string {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		panic(err)
	}
	return `{
		"providers": {
			"xai": {
				"id": "xai",
				"oauth": ` + string(tokenJSON) + `,
				"grok_models": [{"id": "grok-stale", "name": "Grok Stale"}]
			}
		}
	}`
}

// stubGrokModels replaces the Grok catalog fetch with one that reports
// whether it ran and what it returns.
func stubGrokModels(t *testing.T, models []catwalk.Model) *bool {
	t.Helper()

	originalFetch := fetchGrokModels
	called := false
	fetchGrokModels = func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
		called = true
		return models, nil
	}
	t.Cleanup(func() { fetchGrokModels = originalFetch })
	return &called
}

// TestLoadRefreshesGrokModelsWhenCatwalkUpdates covers the startup
// wiring: when Catwalk delivers a fresh catalog, the Grok model catalog
// is refreshed in the same run, and the config Load publishes afterwards
// still carries the resolved default models (the refresh swaps the
// config out from under Load, which must re-read it).
func TestLoadRefreshesGrokModelsWhenCatwalkUpdates(t *testing.T) {
	// Not parallel: swaps package-level provider state and the fetch
	// stub, and sets environment variables.
	workDir, dataDir := isolateLoadEnv(t)
	newCatwalkStub(t, http.StatusOK, []catwalk.Provider{{
		Name:                "xAI",
		ID:                  catwalk.InferenceProviderXAI,
		Type:                catwalk.TypeOpenAICompat,
		DefaultLargeModelID: "grok-4.5",
		DefaultSmallModelID: "grok-4.5",
		Models: []catwalk.Model{
			{ID: "grok-4.5", Name: "Grok 4.5"},
		},
	}})
	resetProviderState()
	t.Cleanup(resetProviderState)

	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "crush.json"),
		[]byte(grokLoginConfig(chatGPTLoginToken())),
		0o600,
	))

	fetched := stubGrokModels(t, []catwalk.Model{{ID: "grok-fresh", Name: "Grok Fresh"}})

	store, err := Load(workDir, dataDir, false)
	require.NoError(t, err)

	require.True(t, CatwalkUpdated(), "the stub served a fresh catalog")
	require.True(t, *fetched, "the Grok catalog refresh rides the Catwalk update")

	pc, ok := store.Config().Providers.Get("xai")
	require.True(t, ok)
	require.Equal(t, "grok-fresh", pc.GrokModels[0].ID, "the stale catalog is replaced")

	large, ok := store.Config().Models[SelectedModelTypeLarge]
	require.True(t, ok)
	require.Equal(t, "grok-4.5", large.Model, "the resolved default model survives the catalog refresh")
}

// TestLoadKeepsGrokModelsWhenCatwalkNotModified verifies the refresh
// only follows actual Catwalk updates: a 304 Not Modified leaves the
// persisted catalog alone.
func TestLoadKeepsGrokModelsWhenCatwalkNotModified(t *testing.T) {
	// Not parallel: swaps package-level provider state and the fetch
	// stub, and sets environment variables.
	workDir, dataDir := isolateLoadEnv(t)
	newCatwalkStub(t, http.StatusNotModified, nil)
	resetProviderState()
	t.Cleanup(resetProviderState)

	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "crush.json"),
		[]byte(grokLoginConfig(chatGPTLoginToken())),
		0o600,
	))

	fetched := stubGrokModels(t, []catwalk.Model{{ID: "grok-fresh", Name: "Grok Fresh"}})

	store, err := Load(workDir, dataDir, false)
	require.NoError(t, err)

	require.False(t, CatwalkUpdated(), "the stub reported the catalog unchanged")
	require.False(t, *fetched, "an unchanged Catwalk catalog does not trigger a refresh")

	pc, ok := store.Config().Providers.Get("xai")
	require.True(t, ok)
	require.Equal(t, "grok-stale", pc.GrokModels[0].ID, "the persisted catalog is kept")
}
