package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestGetModelIncludesCopilotModels(t *testing.T) {
	t.Parallel()

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("copilot", ProviderConfig{
		Models:             []catwalk.Model{{ID: "gpt-4.1"}},
		SubscriptionModels: []catwalk.Model{{ID: "claude-sonnet-4.5", Name: "Claude Sonnet 4.5"}},
	})
	cfg := &Config{Providers: providers}

	model := cfg.GetModel("copilot", "claude-sonnet-4.5")
	require.NotNil(t, model)
	require.Equal(t, "Claude Sonnet 4.5", model.Name)

	require.NotNil(t, cfg.GetModel("copilot", "gpt-4.1"))
	require.Nil(t, cfg.GetModel("copilot", "missing"))
}

// TestSetProviderAPIKeyCopilot proves a Copilot login fetches the model
// catalog the subscription grants, and that entering an API key retires
// the login together with its catalog.
func TestSetProviderAPIKeyCopilot(t *testing.T) {
	// Not parallel: t.Setenv below.

	// Point config discovery at the test sandbox: the write below
	// triggers an auto-reload, which must not pick up the developer's
	// real crush.json.
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	token := &oauth.Token{
		AccessToken:  "copilot-at",
		RefreshToken: "copilot-rt",
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

		stubSubscriptionFetch(t, "copilot", func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
			return []catwalk.Model{{ID: "claude-sonnet-4.5"}}, nil
		})

		return &ConfigStore{
			config:         &Config{Providers: providers},
			globalDataPath: configPath,
			workingDir:     dir,
		}
	}

	t.Run("login fetches the subscription catalog", func(t *testing.T) {
		store := newStore(t, `{"providers":{"copilot":{"id":"copilot","extra_headers":{}}}}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "copilot", token))

		pc, ok := store.Config().Providers.Get("copilot")
		require.True(t, ok)
		require.Equal(t, token, pc.OAuthToken)
		require.Equal(t, "claude-sonnet-4.5", pc.SubscriptionModels[0].ID, "the subscription catalog lands in its own field")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.Contains(t, string(disk), "claude-sonnet-4.5", "the catalog is persisted")
	})

	t.Run("api key keeps the copilot login", func(t *testing.T) {
		store := newStore(t, `{
			"providers": {
				"copilot": {
					"id": "copilot",
					"oauth": {"access_token": "old-at", "refresh_token": "old-rt"},
					"subscription_models": [{"id": "claude-sonnet-4.5"}]
				}
			}
		}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "copilot", "sk-new"))

		pc, ok := store.Config().Providers.Get("copilot")
		require.True(t, ok)
		require.Equal(t, "sk-new", pc.APIKey)
		require.NotNil(t, pc.OAuthToken, "the key mirrors the login's token, so the login stays to refresh it")
		require.Equal(t, "claude-sonnet-4.5", pc.SubscriptionModels[0].ID)

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.Contains(t, string(disk), "old-rt")
	})

	t.Run("new login drops the previous login's catalog", func(t *testing.T) {
		store := newStore(t, `{
			"providers": {
				"copilot": {
					"id": "copilot",
					"oauth": {"access_token": "old-at", "refresh_token": "old-rt"},
					"subscription_models": [{"id": "old-account-model"}]
				}
			}
		}`)
		stubSubscriptionFetch(t, "copilot", func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
			return nil, errors.New("offline")
		})

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "copilot", token))

		pc, ok := store.Config().Providers.Get("copilot")
		require.True(t, ok)
		require.Empty(t, pc.SubscriptionModels, "a failed fetch must not leave the old account's catalog")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.NotContains(t, string(disk), "old-account-model")
	})
}

// TestRefetchCopilotModelsCopilot proves the Copilot catalog
// follows the shared policy: fetched lazily when missing, left alone once
// present, and never committed onto a different login.
func TestRefetchSubscriptionModelsCopilot(t *testing.T) {
	// Not parallel: t.Setenv below, and it stubs the catalog fetch.

	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	token := &oauth.Token{
		AccessToken:  "copilot-at",
		RefreshToken: "github-token",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	}

	newStore := func(t *testing.T, pc ProviderConfig, fetch func(context.Context, *oauth.Token) ([]catwalk.Model, error)) *ConfigStore {
		t.Helper()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "crush.json")
		require.NoError(t, os.WriteFile(configPath, []byte(`{"providers":{"copilot":{"id":"copilot"}}}`), 0o600))

		pc.ID = "copilot"
		providers := csync.NewMap[string, ProviderConfig]()
		providers.Set("copilot", pc)

		stubSubscriptionFetch(t, "copilot", fetch)

		return &ConfigStore{
			config:         &Config{Providers: providers},
			globalDataPath: configPath,
			workingDir:     dir,
		}
	}
	counting := func(calls *atomic.Int32) func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
		return func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
			calls.Add(1)
			return []catwalk.Model{{ID: "fresh-model"}}, nil
		}
	}

	t.Run("existing catalog is a no-op", func(t *testing.T) {
		var calls atomic.Int32
		store := newStore(t, ProviderConfig{OAuthToken: token, SubscriptionModels: []catwalk.Model{{ID: "kept-model"}}}, counting(&calls))

		store.RefetchSubscriptionModels(t.Context())

		require.Equal(t, int32(0), calls.Load())
		pc, _ := store.Config().Providers.Get("copilot")
		require.Equal(t, "kept-model", pc.SubscriptionModels[0].ID)
	})

	t.Run("missing catalog is fetched", func(t *testing.T) {
		var calls atomic.Int32
		store := newStore(t, ProviderConfig{OAuthToken: token}, counting(&calls))

		store.RefetchSubscriptionModels(t.Context())

		require.Equal(t, int32(1), calls.Load())
		pc, _ := store.Config().Providers.Get("copilot")
		require.Equal(t, "fresh-model", pc.SubscriptionModels[0].ID)
	})

	t.Run("signed out is a no-op", func(t *testing.T) {
		var calls atomic.Int32
		store := newStore(t, ProviderConfig{}, counting(&calls))

		store.RefetchSubscriptionModels(t.Context())

		require.Equal(t, int32(0), calls.Load())
	})

	t.Run("expired token is refreshed before the fetch", func(t *testing.T) {
		expired := &oauth.Token{
			AccessToken:  "old-at",
			RefreshToken: "github-token",
			ExpiresIn:    3600,
			ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
		}
		fresh := &oauth.Token{
			AccessToken:  "fresh-at",
			RefreshToken: "github-token",
			ExpiresIn:    3600,
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		}

		var fetchToken atomic.Value
		store := newStore(t, ProviderConfig{OAuthToken: expired, ExtraHeaders: map[string]string{}},
			func(_ context.Context, token *oauth.Token) ([]catwalk.Model, error) {
				fetchToken.Store(token.AccessToken)
				return []catwalk.Model{{ID: "fresh-model"}}, nil
			})
		var exchanges atomic.Int32
		store.exchangeToken = func(_ context.Context, providerID, refreshToken string) (*oauth.Token, error) {
			exchanges.Add(1)
			require.Equal(t, "copilot", providerID)
			require.Equal(t, "github-token", refreshToken)
			return fresh, nil
		}

		store.RefetchSubscriptionModels(t.Context())

		require.Equal(t, int32(1), exchanges.Load(), "the expired token is exchanged")
		require.Equal(t, "fresh-at", fetchToken.Load(), "the fetch uses the refreshed token")
		pc, _ := store.Config().Providers.Get("copilot")
		require.Equal(t, fresh, pc.OAuthToken)
		require.Equal(t, "fresh-model", pc.SubscriptionModels[0].ID)
	})

	// swapTokenDuringFetch replaces the stored token while the fetch is in
	// flight, the way a concurrent refresh or login would.
	swapTokenDuringFetch := func(t *testing.T, replacement *oauth.Token) *ConfigStore {
		t.Helper()
		var store *ConfigStore
		store = newStore(t, ProviderConfig{OAuthToken: token}, func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
			pc, _ := store.Config().Providers.Get("copilot")
			pc.OAuthToken = replacement
			store.Config().Providers.Set("copilot", pc)
			return []catwalk.Model{{ID: "fresh-model"}}, nil
		})
		store.RefetchSubscriptionModels(t.Context())
		return store
	}

	t.Run("a different login during the fetch discards the result", func(t *testing.T) {
		store := swapTokenDuringFetch(t, &oauth.Token{AccessToken: "other-at", RefreshToken: "other-github-token"})

		pc, _ := store.Config().Providers.Get("copilot")
		require.Empty(t, pc.SubscriptionModels, "the old login's catalog must not land on the new login")
	})

	t.Run("an access token rotation during the fetch keeps the result", func(t *testing.T) {
		store := swapTokenDuringFetch(t, &oauth.Token{AccessToken: "rotated-at", RefreshToken: "github-token"})

		pc, _ := store.Config().Providers.Get("copilot")
		require.Equal(t, "fresh-model", pc.SubscriptionModels[0].ID, "a refresh does not change the login")
	})
}

func TestSubscriptionCatalogLookups(t *testing.T) {
	t.Parallel()

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("copilot", ProviderConfig{
		ID:                 "copilot",
		Models:             []catwalk.Model{{ID: "gpt-4.1", Name: "Static"}},
		SubscriptionModels: []catwalk.Model{{ID: "gpt-4.1", Name: "Scoped"}, {ID: "claude-sonnet-4.5"}},
	})
	cfg := &Config{Providers: providers}

	require.True(t, cfg.IsModelAvailable("copilot", "claude-sonnet-4.5"), "subscription models are available")
	require.False(t, cfg.IsModelAvailable("copilot", "missing"))

	pc, _ := providers.Get("copilot")
	require.Equal(t, "Static", pc.ActiveModels()[0].Name, "signed out, the static catalog is active")
	require.Nil(t, pc.SignedInModels())

	cfg.GetModel("copilot", "claude-sonnet-4.5").Name = "mutated"
	require.Empty(t, cfg.GetModel("copilot", "claude-sonnet-4.5").Name, "lookups return copies")

	pc.OAuthToken = &oauth.Token{AccessToken: "at"}
	require.Len(t, pc.ActiveModels(), 2, "signed in, the subscription catalog is active")
	require.Equal(t, "Scoped", pc.ActiveModels()[0].Name)
}
