package config

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// TestRefreshOAuthToken_DeclaredProvider proves the plugin path end to
// end: a provider that declares its own OAuth endpoints refreshes through
// them, with no Crush code knowing the server exists.
func TestRefreshOAuthToken_DeclaredProvider(t *testing.T) {
	t.Parallel()

	var grants []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		grants = append(grants, r.PostForm.Get("grant_type"))
		require.Equal(t, "crush-example", r.PostForm.Get("client_id"))
		require.Equal(t, "old-refresh", r.PostForm.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"fresh-access","refresh_token":"fresh-refresh","expires_in":3600}`)
	}))
	defer server.Close()

	expired := &oauth.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")
	require.NoError(t, os.WriteFile(configPath, fmt.Appendf(nil, `{
		"providers": {
			"example": {
				"api_key": %q,
				"oauth": {
					"access_token": %q,
					"refresh_token": %q,
					"expires_in": %d,
					"expires_at": %d
				}
			}
		}
	}`, expired.AccessToken, expired.AccessToken, expired.RefreshToken,
		expired.ExpiresIn, expired.ExpiresAt), 0o600))

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("example", ProviderConfig{
		ID:         "example",
		Name:       "Example",
		APIKey:     expired.AccessToken,
		OAuthToken: expired,
		Auth: &oauth.AuthSpec{
			ClientID: "crush-example",
			TokenURL: server.URL + "/token",
		},
	})

	store := &ConfigStore{
		config:         &Config{Providers: providers},
		globalDataPath: configPath,
	}

	require.NoError(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "example"))
	require.Equal(t, []string{"refresh_token"}, grants)

	updated, ok := store.config.Providers.Get("example")
	require.True(t, ok)
	require.Equal(t, "fresh-access", updated.OAuthToken.AccessToken)
	require.Equal(t, "fresh-refresh", updated.OAuthToken.RefreshToken)
	require.Equal(
		t,
		"fresh-access",
		updated.APIKey,
		"the access token mirrors into api_key so bearer requests use the fresh credential",
	)

	// The renewed credential must survive a restart, so it belongs on disk.
	onDisk, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(onDisk), "fresh-access")
	require.NotContains(t, string(onDisk), "old-access")
}

// TestRefreshOAuthToken_UnknownProviderStillRefuses keeps the old failure
// for a provider that neither Crush nor the config knows how to refresh.
func TestRefreshOAuthToken_UnknownProviderStillRefuses(t *testing.T) {
	t.Parallel()

	expired := &oauth.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
	}

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("plain", ProviderConfig{ID: "plain", OAuthToken: expired})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"providers":{"plain":{}}}`), 0o600))

	store := &ConfigStore{
		config:         &Config{Providers: providers},
		globalDataPath: configPath,
	}

	err := store.RefreshOAuthToken(context.Background(), ScopeGlobal, "plain")
	require.ErrorContains(t, err, "OAuth refresh not supported for provider plain")
}

// A provider that declares an OAuth flow but has not signed in yet keeps its
// models in the picker, but must never become the default selection: there is
// no credential to build a client with, and some SDKs refuse to construct one
// at all, which would fail every startup until the sign-in runs.
func TestDefaultModelSelectionSkipsSignedOutProviders(t *testing.T) {
	t.Parallel()

	signedOut := ProviderConfig{
		ID:      "gemini-sub",
		BaseURL: "https://generativelanguage.googleapis.com/",
		Auth:    &oauth.AuthSpec{TokenURL: "https://oauth2.googleapis.com/token"},
		Models:  []catwalk.Model{{ID: "gemini-3.8-flash", DefaultMaxTokens: 65536}},
	}
	cfg := &Config{Providers: csync.NewMapFrom(map[string]ProviderConfig{
		"gemini-sub": signedOut,
		"openai": {
			ID:     "openai",
			APIKey: "sk-test",
			Models: []catwalk.Model{{ID: "gpt-x", DefaultMaxTokens: 1000}},
		},
	})}

	large, _, err := cfg.defaultModelSelection(nil)
	require.NoError(t, err)
	require.Equal(t, "openai", large.Provider, "the signed-out provider must not be selected")

	// With only signed-out providers there is no default yet, and that is
	// not an error: the picker still lists the models, awaiting a sign-in.
	only := &Config{Providers: csync.NewMapFrom(map[string]ProviderConfig{"gemini-sub": signedOut})}
	large, _, err = only.defaultModelSelection(nil)
	require.NoError(t, err)
	require.Empty(t, large.Provider)
	require.True(t, only.IsConfigured(), "a signed-out provider is still configured")
	require.False(t, only.HasUsableSelection())
}

// The "configured" badge means a provider can serve a request now, so a
// declared OAuth flow without a credential is not configured until the
// sign-in lands, whichever way the credential arrives.
func TestNeedsSignIn(t *testing.T) {
	t.Parallel()

	signedOut := ProviderConfig{Auth: &oauth.AuthSpec{TokenURL: "https://example.com/token"}}
	require.True(t, signedOut.NeedsSignIn())

	withToken := ProviderConfig{
		Auth:       &oauth.AuthSpec{TokenURL: "https://example.com/token"},
		OAuthToken: &oauth.Token{AccessToken: "at"},
	}
	require.False(t, withToken.NeedsSignIn())

	withKey := ProviderConfig{
		Auth:   &oauth.AuthSpec{TokenURL: "https://example.com/token"},
		APIKey: "sk-test",
	}
	require.False(t, withKey.NeedsSignIn())

	// A provider with no OAuth flow needs no sign-in: a local model server
	// has no credential at all and still serves.
	local := ProviderConfig{BaseURL: "http://localhost:11434/v1"}
	require.False(t, local.NeedsSignIn())
}
