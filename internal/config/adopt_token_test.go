package config

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// noExchange fails the test if an exchange is attempted. Adoption is meant
// to settle entirely from disk, and spending a refresh token to reach the
// same place is the bug these tests exist to catch.
func noExchange(t *testing.T) func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error) {
	t.Helper()
	return func(context.Context, string, string) (*oauth.Token, error) {
		t.Fatal("adoption must not perform a token exchange")
		return nil, nil
	}
}

// TestAdoptNewerDiskToken_TakesPeerToken covers the idle-session case: our
// in-memory credential aged out while a peer rotated the family forward, so
// the live token is the one on disk and it should be taken as-is.
func TestAdoptNewerDiskToken_TakesPeerToken(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))

	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "peer-at",
		RefreshToken: "peer-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})

	adopted, err := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.True(t, adopted)

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "peer-at", pc.OAuthToken.AccessToken)
	require.Equal(t, "peer-rt", pc.OAuthToken.RefreshToken)
	require.Equal(t, "peer-at", pc.APIKey, "the mirrored API key follows the token")
}

// TestAdoptNewerDiskToken_IgnoresExpiredPeerToken keeps adoption to tokens
// that can actually be used. An expired peer token is still worth borrowing
// a refresh token from, but that is the refresh path's job, not this one's.
func TestAdoptNewerDiskToken_IgnoresExpiredPeerToken(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))

	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "peer-at",
		RefreshToken: "peer-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(-time.Minute).Unix(),
	})

	adopted, err := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.False(t, adopted)

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "at0", pc.OAuthToken.AccessToken, "the in-memory token stands")
}

// TestAdoptNewerDiskToken_FollowsRefreshTokenNotExpiry pins the rule that
// decides rotation: a different refresh token on disk means a peer rotated,
// even when its access token was issued with less time on it than ours.
// Judging by expiry instead would keep a retired refresh token in use.
func TestAdoptNewerDiskToken_FollowsRefreshTokenNotExpiry(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newStoreHoldingToken(t, configPath, &oauth.Token{
		AccessToken:  "at0",
		RefreshToken: "rt0",
		ExpiresIn:    7200,
		ExpiresAt:    time.Now().Add(2 * time.Hour).Unix(),
	}, noExchange(t))

	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "peer-at",
		RefreshToken: "peer-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})

	adopted, err := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.True(t, adopted)

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "peer-rt", pc.OAuthToken.RefreshToken)
}

// TestAdoptNewerDiskToken_IgnoresOlderAccessTokenForSameRefreshToken keeps
// providers that never rotate refresh tokens from walking backwards onto an
// older access token.
func TestAdoptNewerDiskToken_IgnoresOlderAccessTokenForSameRefreshToken(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newStoreHoldingToken(t, configPath, &oauth.Token{
		AccessToken:  "at0",
		RefreshToken: "rt0",
		ExpiresIn:    7200,
		ExpiresAt:    time.Now().Add(2 * time.Hour).Unix(),
	}, noExchange(t))

	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "older-at",
		RefreshToken: "rt0",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})

	adopted, err := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.False(t, adopted)
}

// TestAdoptNewerDiskToken_ReportsUnreadableDisk keeps a failed read from
// passing for "nothing newer on disk".
func TestAdoptNewerDiskToken_ReportsUnreadableDisk(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))
	writeCorruptTokenToDisk(t, configPath)

	adopted, err := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.Error(t, err)
	require.False(t, adopted)
}

// competingToken is the credential a refresh or sign-in lands while an
// adoption is between reading disk and applying what it read.
var competingToken = &oauth.Token{
	AccessToken:  "competing-at",
	RefreshToken: "competing-rt",
	ExpiresIn:    3600,
	ExpiresAt:    time.Now().Add(time.Hour).Unix(),
}

// TestAdoptNewerDiskToken_DoesNotRollBackAConcurrentWrite lands a new
// credential in the window between reading disk and applying it. The
// adoption has to notice and stand down instead of replacing the newer
// credential with what disk held when it looked.
func TestAdoptNewerDiskToken_DoesNotRollBackAConcurrentWrite(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))
	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "peer-at",
		RefreshToken: "peer-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})
	store.beforeAdoptApply = func() { store.applyToken("hyper", competingToken) }

	adopted, err := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.False(t, adopted)

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "competing-at", pc.OAuthToken.AccessToken)
	require.Equal(t, "competing-at", pc.APIKey)
}

// TestAdoptPeerRefreshedToken_DoesNotRollBackAConcurrentWrite is the same
// window on the path taken after a rejected exchange.
func TestAdoptPeerRefreshedToken_DoesNotRollBackAConcurrentWrite(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))
	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "peer-at",
		RefreshToken: "peer-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})
	store.beforeAdoptApply = func() { store.applyToken("hyper", competingToken) }

	adopted, err := store.AdoptPeerRefreshedToken(context.Background(), ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.False(t, adopted)

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "competing-at", pc.OAuthToken.AccessToken)
}

// TestAdoptPeerRefreshedToken_RecoversFromRejectedExchange is the case that
// costs a user their session: our exchange came back rejected because a peer
// had already rotated the family, and its replacement is sitting on disk.
// Finding it here is the difference between carrying on and a sign-in prompt.
func TestAdoptPeerRefreshedToken_RecoversFromRejectedExchange(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))

	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "winner-at",
		RefreshToken: "winner-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})

	adopted, err := store.AdoptPeerRefreshedToken(context.Background(), ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.True(t, adopted)

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "winner-at", pc.OAuthToken.AccessToken)
	require.Equal(t, "winner-rt", pc.OAuthToken.RefreshToken)
	require.Equal(t, "winner-at", pc.APIKey, "the mirrored API key follows the token")
}

// TestAdoptPeerRefreshedToken_ReportsGenuineRevocation keeps the recovery
// path from masking the real thing. With nothing newer on disk there is no
// peer to defer to, so the caller must go on to ask for a sign-in.
func TestAdoptPeerRefreshedToken_ReportsGenuineRevocation(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))

	adopted, err := store.AdoptPeerRefreshedToken(context.Background(), ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.False(t, adopted)
}

// TestAdoptPeerRefreshedToken_IgnoresExpiredPeerToken keeps adoption to
// tokens that can actually be used. An expired peer token is still worth
// borrowing a refresh token from, but that is the refresh path's job.
func TestAdoptPeerRefreshedToken_IgnoresExpiredPeerToken(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))

	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "peer-at",
		RefreshToken: "peer-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(-time.Minute).Unix(),
	})

	adopted, err := store.AdoptPeerRefreshedToken(context.Background(), ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.False(t, adopted)
}
