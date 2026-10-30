package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// stubSubscriptionFetch replaces the provider's catalog fetch for the test.
func stubSubscriptionFetch(t *testing.T, providerID string, fetch func(context.Context, *oauth.Token) ([]catwalk.Model, error)) {
	t.Helper()
	original := subscriptions[providerID]
	stub := original
	stub.fetch = fetch
	subscriptions[providerID] = stub
	t.Cleanup(func() { subscriptions[providerID] = original })
}

func TestMigrateLegacySubscriptionModels(t *testing.T) {
	t.Parallel()

	in := `{"providers":{
		"openai":{"chatgpt_models":[{"id":"gpt-codex","cost_per_1m_in":1}]},
		"xai":{"grok_models":[{"id":"grok-old"}],"subscription_models":[{"id":"grok-new"}]},
		"copilot":{"copilot_models":[{"id":"claude"}]}
	}}`
	out, changed := migrateLegacyConfig([]byte(in))
	require.True(t, changed)

	var cfg struct {
		Providers map[string]map[string]json.RawMessage `json:"providers"`
	}
	require.NoError(t, json.Unmarshal(out, &cfg))
	for provider, legacy := range map[string]string{"openai": "chatgpt_models", "xai": "grok_models", "copilot": "copilot_models"} {
		require.NotContains(t, cfg.Providers[provider], legacy, "%s's legacy key is removed", provider)
	}
	require.JSONEq(t, `[{"id":"gpt-codex","pricing":{"input":1}}]`, string(cfg.Providers["openai"]["subscription_models"]),
		"moved catalogs get the catwalk v3 model migration too")
	require.JSONEq(t, `[{"id":"grok-new"}]`, string(cfg.Providers["xai"]["subscription_models"]), "an existing catalog wins")
	require.JSONEq(t, `[{"id":"claude"}]`, string(cfg.Providers["copilot"]["subscription_models"]))

	_, changed = migrateLegacyConfig(out)
	require.False(t, changed, "the migration is idempotent")
}

func TestSetConfigFieldsDeletesInTheSameWrite(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())

	dir := t.TempDir()
	path := filepath.Join(dir, "crush.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"providers":{"x":{"oauth":{"access_token":"at"},"api_key":"old"}}}`), 0o600))
	store := &ConfigStore{config: &Config{Providers: csync.NewMap[string, ProviderConfig]()}, globalDataPath: path, workingDir: dir}

	require.NoError(t, store.SetConfigFields(ScopeGlobal, map[string]any{
		"providers.x.api_key": "new",
		"providers.x.oauth":   deletedField{},
	}))

	disk, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{"providers":{"x":{"api_key":"new"}}}`, string(disk))
}

// newSubscriptionStore builds a store whose provider is signed in with the
// given token and carries an old catalog.
func newSubscriptionStore(t *testing.T, providerID string, token *oauth.Token) *ConfigStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "crush.json")
	tokenJSON, err := json.Marshal(token)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{"providers":{"`+providerID+`":{"id":"`+providerID+
		`","oauth":`+string(tokenJSON)+`,"subscription_models":[{"id":"old-model"}]}}}`), 0o600))

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set(providerID, ProviderConfig{
		ID:                 providerID,
		OAuthToken:         token,
		ExtraHeaders:       map[string]string{},
		SubscriptionModels: []catwalk.Model{{ID: "old-model"}},
	})
	return &ConfigStore{config: &Config{Providers: providers}, globalDataPath: path, workingDir: dir}
}

// TestSubscriptionPolicyIsShared pins the behavior every subscription
// provider gets from the shared catalog code.
func TestSubscriptionPolicyIsShared(t *testing.T) {
	// Not parallel: t.Setenv below, and it stubs the catalog fetch.
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	for providerID := range subscriptions {
		t.Run(providerID+" failed refresh skips the fetch", func(t *testing.T) {
			store := newSubscriptionStore(t, providerID, &oauth.Token{
				AccessToken: "old-at", RefreshToken: "old-rt", ExpiresAt: time.Now().Add(-time.Hour).Unix(),
			})
			store.exchangeToken = func(context.Context, string, string) (*oauth.Token, error) {
				return nil, errors.New("revoked")
			}
			fetched := false
			stubSubscriptionFetch(t, providerID, func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
				fetched = true
				return []catwalk.Model{{ID: "new-model"}}, nil
			})

			store.refetchSubscriptionModels(t.Context(), ScopeGlobal, providerID)

			require.False(t, fetched, "an expired token would only earn a 401")
			pc, _ := store.Config().Providers.Get(providerID)
			require.Equal(t, "old-model", pc.SubscriptionModels[0].ID, "the existing catalog is kept")
		})

		t.Run(providerID+" new login drops the previous catalog", func(t *testing.T) {
			store := newSubscriptionStore(t, providerID, &oauth.Token{AccessToken: "old-at", RefreshToken: "old-rt"})
			stubSubscriptionFetch(t, providerID, func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
				return nil, errors.New("offline")
			})

			newLogin := &oauth.Token{AccessToken: "new-at", RefreshToken: "new-rt", ExpiresAt: time.Now().Add(time.Hour).Unix()}
			require.NoError(t, store.SetProviderAPIKey(ScopeGlobal, providerID, newLogin))

			pc, _ := store.Config().Providers.Get(providerID)
			require.Empty(t, pc.SubscriptionModels, "the old account's catalog must not outlive its login")
			disk, err := os.ReadFile(store.globalDataPath)
			require.NoError(t, err)
			require.NotContains(t, string(disk), "old-model")
		})
	}
}

// TestRefreshKeepsAccountID proves a refresh whose response omits the ID
// token keeps naming the same account, so a catalog fetch racing the
// refresh is not discarded.
func TestRefreshKeepsAccountID(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	store := newSubscriptionStore(t, "xai", &oauth.Token{
		AccessToken: "old-at", RefreshToken: "old-rt", AccountID: "user-1", ExpiresAt: time.Now().Add(-time.Hour).Unix(),
	})
	store.exchangeToken = func(context.Context, string, string) (*oauth.Token, error) {
		return &oauth.Token{AccessToken: "new-at", RefreshToken: "rotated-rt", ExpiresAt: time.Now().Add(time.Hour).Unix()}, nil
	}

	require.NoError(t, store.RefreshOAuthToken(t.Context(), ScopeGlobal, "xai"))

	pc, _ := store.Config().Providers.Get("xai")
	require.Equal(t, "rotated-rt", pc.OAuthToken.RefreshToken)
	require.Equal(t, "user-1", pc.OAuthToken.AccountID)
}
