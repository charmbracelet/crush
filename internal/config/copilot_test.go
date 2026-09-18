package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestGetModelIncludesCopilotModels(t *testing.T) {
	t.Parallel()

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("copilot", ProviderConfig{
		Models:        []catwalk.Model{{ID: "gpt-4.1"}},
		CopilotModels: []catwalk.Model{{ID: "claude-sonnet-4.5", Name: "Claude Sonnet 4.5"}},
	})
	cfg := &Config{Providers: providers}

	model := cfg.GetModel("copilot", "claude-sonnet-4.5")
	require.NotNil(t, model)
	require.Equal(t, "Claude Sonnet 4.5", model.Name)

	require.NotNil(t, cfg.GetModel("copilot", "gpt-4.1"))
	require.Nil(t, cfg.GetModel("copilot", "missing"))
}

func TestIsModelAvailableIncludesCopilotModels(t *testing.T) {
	t.Parallel()

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("copilot", ProviderConfig{
		Models:        []catwalk.Model{{ID: "gpt-4.1"}},
		CopilotModels: []catwalk.Model{{ID: "auto", Name: "Auto"}},
	})
	cfg := &Config{Providers: providers}

	require.True(t, cfg.IsModelAvailable("copilot", "gpt-4.1"))
	require.True(t, cfg.IsModelAvailable("copilot", "auto"))
	require.False(t, cfg.IsModelAvailable("copilot", "missing"))
	require.False(t, cfg.IsModelAvailable("missing-provider", "auto"))
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

		return &ConfigStore{
			config:         &Config{Providers: providers},
			globalDataPath: configPath,
			workingDir:     dir,
			fetchCopilotModels: func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
				return []catwalk.Model{{ID: "claude-sonnet-4.5"}}, nil
			},
		}
	}

	t.Run("login fetches the subscription catalog", func(t *testing.T) {
		store := newStore(t, `{"providers":{"copilot":{"id":"copilot","extra_headers":{}}}}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "copilot", token))

		pc, ok := store.Config().Providers.Get("copilot")
		require.True(t, ok)
		require.Equal(t, token, pc.OAuthToken)
		require.Equal(t, "claude-sonnet-4.5", pc.CopilotModels[0].ID, "the subscription catalog lands in its own field")
		require.False(t, pc.CopilotModelsFetchAt.IsZero(), "the fetch is timestamped")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.Contains(t, string(disk), "claude-sonnet-4.5", "the catalog is persisted")
	})

	t.Run("api key retires the copilot login", func(t *testing.T) {
		store := newStore(t, `{
			"providers": {
				"copilot": {
					"id": "copilot",
					"oauth": {"access_token": "old-at", "refresh_token": "old-rt"},
					"copilot_models": [{"id": "claude-sonnet-4.5"}],
					"copilot_models_fetch_at": "2026-01-01T00:00:00Z"
				}
			}
		}`)

		require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, "copilot", "sk-new"))

		pc, ok := store.Config().Providers.Get("copilot")
		require.True(t, ok)
		require.Equal(t, "sk-new", pc.APIKey)
		require.Nil(t, pc.OAuthToken, "the API key replaces the Copilot login")
		require.Empty(t, pc.CopilotModels, "the subscription catalog goes with it")
		require.True(t, pc.CopilotModelsFetchAt.IsZero(), "the fetch timestamp goes with it")

		disk, err := os.ReadFile(store.globalDataPath)
		require.NoError(t, err)
		require.NotContains(t, string(disk), "old-rt", "the retired login is gone from the config file")
		require.NotContains(t, string(disk), "claude-sonnet-4.5")
	})
}

// TestRefetchCopilotModelsTTL proves the catalog is refreshed only once
// it grows stale: a fresh catalog is left alone, a stale one is replaced.
func TestRefetchCopilotModelsTTL(t *testing.T) {
	// Not parallel: t.Setenv below.

	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	token := &oauth.Token{
		AccessToken: "copilot-at",
		ExpiresIn:   3600,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}

	newStore := func(t *testing.T, fetchAt time.Time, calls *atomic.Int32) *ConfigStore {
		t.Helper()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "crush.json")
		require.NoError(t, os.WriteFile(configPath, []byte(`{"providers":{"copilot":{"id":"copilot"}}}`), 0o600))

		providers := csync.NewMap[string, ProviderConfig]()
		providers.Set("copilot", ProviderConfig{
			ID:                   "copilot",
			OAuthToken:           token,
			CopilotModels:        []catwalk.Model{{ID: "stale-model"}},
			CopilotModelsFetchAt: fetchAt,
		})

		return &ConfigStore{
			config:         &Config{Providers: providers},
			globalDataPath: configPath,
			workingDir:     dir,
			fetchCopilotModels: func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
				calls.Add(1)
				return []catwalk.Model{{ID: "fresh-model"}}, nil
			},
		}
	}

	t.Run("fresh catalog is a no-op", func(t *testing.T) {
		var calls atomic.Int32
		store := newStore(t, time.Now(), &calls)

		store.RefetchCopilotModels(context.Background())

		require.Equal(t, int32(0), calls.Load(), "a catalog younger than the TTL is trusted")
		pc, _ := store.Config().Providers.Get("copilot")
		require.Equal(t, "stale-model", pc.CopilotModels[0].ID)
	})

	t.Run("stale catalog is refreshed", func(t *testing.T) {
		var calls atomic.Int32
		store := newStore(t, time.Now().Add(-2*copilotModelsTTL), &calls)

		store.RefetchCopilotModels(context.Background())

		require.Equal(t, int32(1), calls.Load(), "a catalog older than the TTL is refetched")
		pc, _ := store.Config().Providers.Get("copilot")
		require.Equal(t, "fresh-model", pc.CopilotModels[0].ID)
	})

	t.Run("missing catalog is fetched", func(t *testing.T) {
		var calls atomic.Int32
		store := newStore(t, time.Time{}, &calls)

		store.RefetchCopilotModels(context.Background())

		require.Equal(t, int32(1), calls.Load(), "a zero timestamp counts as stale")
	})

	t.Run("expired token is refreshed before the fetch", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "crush.json")
		require.NoError(t, os.WriteFile(configPath, []byte(`{"providers":{"copilot":{"id":"copilot"}}}`), 0o600))

		expired := &oauth.Token{
			AccessToken:  "old-at",
			RefreshToken: "old-rt",
			ExpiresIn:    3600,
			ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
		}
		fresh := &oauth.Token{
			AccessToken:  "fresh-at",
			RefreshToken: "fresh-rt",
			ExpiresIn:    3600,
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		}

		var exchanges atomic.Int32
		var fetchToken atomic.Value
		providers := csync.NewMap[string, ProviderConfig]()
		providers.Set("copilot", ProviderConfig{
			ID:           "copilot",
			OAuthToken:   expired,
			ExtraHeaders: map[string]string{},
		})
		store := &ConfigStore{
			config:         &Config{Providers: providers},
			globalDataPath: configPath,
			workingDir:     dir,
			exchangeToken: func(_ context.Context, providerID, refreshToken string) (*oauth.Token, error) {
				exchanges.Add(1)
				require.Equal(t, "copilot", providerID)
				require.Equal(t, "old-rt", refreshToken)
				return fresh, nil
			},
			fetchCopilotModels: func(_ context.Context, token *oauth.Token) ([]catwalk.Model, error) {
				fetchToken.Store(token.AccessToken)
				return []catwalk.Model{{ID: "fresh-model"}}, nil
			},
		}

		store.RefetchCopilotModels(context.Background())

		require.Equal(t, int32(1), exchanges.Load(), "the expired token is exchanged")
		require.Equal(t, "fresh-at", fetchToken.Load(), "the fetch uses the refreshed token")
		pc, _ := store.Config().Providers.Get("copilot")
		require.Equal(t, fresh, pc.OAuthToken)
		require.Equal(t, "fresh-model", pc.CopilotModels[0].ID)
	})

	t.Run("signed out is a no-op", func(t *testing.T) {
		var calls atomic.Int32
		store := newStore(t, time.Time{}, &calls)
		pc, _ := store.Config().Providers.Get("copilot")
		pc.OAuthToken = nil
		store.Config().Providers.Set("copilot", pc)

		store.RefetchCopilotModels(context.Background())

		require.Equal(t, int32(0), calls.Load())
	})
}
