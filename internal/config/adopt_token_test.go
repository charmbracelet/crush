package config

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func noExchange(t *testing.T) func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error) {
	t.Helper()
	return func(context.Context, string, string) (*oauth.Token, error) {
		t.Fatal("no exchange was expected")
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

// TestAdoptNewerDiskToken_IgnoresOlderDiskToken guards against walking
// backwards onto a credential the provider has already moved past.
func TestAdoptNewerDiskToken_IgnoresOlderDiskToken(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))

	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "ancient",
		RefreshToken: "ancient-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(-24 * time.Hour).Unix(),
	})

	adopted, err := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.False(t, adopted)
}
