package config

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// newStoreHoldingToken builds a ConfigStore whose hyper provider holds
// token, in memory and on disk, so a refresh decides against exactly the
// credential the test describes.
func newStoreHoldingToken(t *testing.T, configPath string, token *oauth.Token, exchange func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error)) *ConfigStore {
	t.Helper()
	writeTokenToDisk(t, configPath, token)

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("hyper", ProviderConfig{
		ID:         "hyper",
		Name:       "Hyper",
		APIKey:     token.AccessToken,
		OAuthToken: token,
	})

	return &ConfigStore{
		config:         &Config{Providers: providers},
		globalDataPath: configPath,
		workingDir:     filepath.Dir(configPath),
		exchangeToken:  exchange,
	}
}

// tokenIssuedAgo builds a token the provider minted ago in the past, with
// the given lifetime.
func tokenIssuedAgo(ago, lifetime time.Duration) *oauth.Token {
	return &oauth.Token{
		AccessToken:  "at0",
		RefreshToken: "rt0",
		ExpiresIn:    int(lifetime.Seconds()),
		ExpiresAt:    time.Now().Add(lifetime - ago).Unix(),
	}
}

// TestRefreshOAuthToken_SkipsJustIssuedCredential covers the burst that
// costs a rotating provider its whole token family: a turn refreshes, and
// seconds later a 401 already in flight, an undecodable error body, and a
// usage poll each ask for a refresh of their own. Every one of those
// exchanges retires the one before it, leaving the other sessions holding
// something the provider has forgotten.
func TestRefreshOAuthToken_SkipsJustIssuedCredential(t *testing.T) {
	isolateHyperCredentials(t)

	var exchanges atomic.Int64
	store := newStoreHoldingToken(t,
		filepath.Join(t.TempDir(), "crush.json"),
		tokenIssuedAgo(2*time.Second, time.Hour),
		func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error) {
			exchanges.Add(1)
			return &oauth.Token{AccessToken: "at1", RefreshToken: "rt1", ExpiresIn: 3600, ExpiresAt: time.Now().Add(time.Hour).Unix()}, nil
		})

	require.NoError(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	require.Zero(t, exchanges.Load(), "a credential issued seconds ago must not be rotated again")

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "rt0", pc.OAuthToken.RefreshToken, "the held credential stays in force")
}

// TestRefreshOAuthToken_ExchangesOlderCredential keeps the cooldown from
// swallowing the refreshes that matter: a credential past its cooldown is
// renewed on request, even with time left on it.
func TestRefreshOAuthToken_ExchangesOlderCredential(t *testing.T) {
	isolateHyperCredentials(t)

	var exchanges atomic.Int64
	store := newStoreHoldingToken(t,
		filepath.Join(t.TempDir(), "crush.json"),
		tokenIssuedAgo(50*time.Minute, time.Hour),
		func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error) {
			exchanges.Add(1)
			return &oauth.Token{AccessToken: "at1", RefreshToken: "rt1", ExpiresIn: 3600, ExpiresAt: time.Now().Add(time.Hour).Unix()}, nil
		})

	require.NoError(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	require.Equal(t, int64(1), exchanges.Load())

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "rt1", pc.OAuthToken.RefreshToken)
}

// TestRefreshOAuthToken_ExchangeSurvivesCallerCancellation is the case
// that turns a shared account into repeated sign-ins.
//
// A refresh is asked for by whatever is running at the time: a turn, or a
// panel poll with a short budget of its own. When that caller goes away
// mid-exchange — the turn ends, the budget lapses — the provider has
// already retired the refresh token we presented, and its replacement is
// dropped on the floor. What stays on disk is dead, and every session
// sharing the account discovers that separately, one forced sign-in each.
func TestRefreshOAuthToken_ExchangeSurvivesCallerCancellation(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var exchangeCtxLived atomic.Bool
	store := newStoreHoldingToken(t, configPath,
		tokenIssuedAgo(2*time.Hour, time.Hour),
		func(exchangeCtx context.Context, providerID, refreshToken string) (*oauth.Token, error) {
			// Stand in for a request in flight when the caller gives up.
			cancel()
			time.Sleep(20 * time.Millisecond)
			exchangeCtxLived.Store(exchangeCtx.Err() == nil)
			return &oauth.Token{AccessToken: "at1", RefreshToken: "rt1", ExpiresIn: 3600, ExpiresAt: time.Now().Add(time.Hour).Unix()}, nil
		})

	require.NoError(t, store.RefreshOAuthToken(ctx, ScopeGlobal, "hyper"))
	require.True(t, exchangeCtxLived.Load(), "the exchange must outlive the caller that asked for it")

	onDisk, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.NotNil(t, onDisk)
	require.Equal(t, "rt1", onDisk.RefreshToken, "the rotated credential must reach disk for peers to adopt")
}
