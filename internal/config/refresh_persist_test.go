package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/lock"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func freshToken(at, rt string) *oauth.Token {
	return &oauth.Token{AccessToken: at, RefreshToken: rt, ExpiresIn: 3600, ExpiresAt: time.Now().Add(time.Hour).Unix()}
}

// A refreshed token reaches disk before it becomes the live credential, so
// a peer reading disk never sees something older than what is in use here.
func TestRefreshOAuthToken_PersistsBeforePublishing(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		return freshToken("at1", "rt1"), nil
	})
	var liveAtWrite string
	store.persistTokenFields = func(scope Scope, fields map[string]any) error {
		pc, _ := store.config.Providers.Get("hyper")
		liveAtWrite = pc.OAuthToken.AccessToken
		return store.SetConfigFields(scope, fields)
	}

	require.NoError(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	require.Equal(t, "at0", liveAtWrite, "the new token must be on disk before it is published")

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "at1", pc.OAuthToken.AccessToken)
}

// When the save fails after an exchange, the provider has already retired
// the refresh token we spent, so the new token stays live in memory. Until
// it reaches disk, later refreshes retry the save and spend nothing, and
// nothing is adopted from the stale disk copy.
func TestRefreshOAuthToken_RetriesAFailedSaveWithoutSpending(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	var exchanges atomic.Int64
	store := newRefreshTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		if exchanges.Add(1) > 1 {
			t.Error("a refresh spent a token while a save was pending")
		}
		return freshToken("at1", "rt1"), nil
	})
	var failWrites atomic.Bool
	failWrites.Store(true)
	store.persistTokenFields = func(scope Scope, fields map[string]any) error {
		if failWrites.Load() {
			return errors.New("disk full")
		}
		return store.SetConfigFields(scope, fields)
	}

	// The exchange succeeds but the save fails: the new token is live.
	require.NoError(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	pc, _ := store.config.Providers.Get("hyper")
	require.Equal(t, "rt1", pc.OAuthToken.RefreshToken)
	onDisk, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.Equal(t, "rt0", onDisk.RefreshToken)

	// Still failing: the refresh reports it and exchanges nothing.
	require.Error(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	adopted, _ := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.False(t, adopted, "the stale disk copy must not be adopted while a save is pending")
	pc, _ = store.config.Providers.Get("hyper")
	require.Equal(t, "rt1", pc.OAuthToken.RefreshToken)

	// The disk recovers: the next refresh saves the pending token first
	// and has nothing left to spend.
	failWrites.Store(false)
	require.NoError(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	require.Equal(t, int64(1), exchanges.Load())
	onDisk, err = store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.Equal(t, "rt1", onDisk.RefreshToken)
	_, pending := store.pendingToken("hyper")
	require.False(t, pending)
}

// A refresh still waiting for the refresh lock when its caller goes away
// (shutdown, for a background renewal) gives up without exchanging.
func TestRefreshOAuthToken_AbortsWhileWaitingForTheLock(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	var exchanges atomic.Int64
	store := newRefreshTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		exchanges.Add(1)
		return freshToken("at1", "rt1"), nil
	})

	release, err := lock.File(context.Background(), store.refreshLockPath("hyper"))
	require.NoError(t, err)
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- store.RefreshOAuthToken(ctx, ScopeGlobal, "hyper") }()
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("the refresh kept waiting for the lock after its caller went away")
	}
	require.Zero(t, exchanges.Load())
}

// A truncated config file says nothing about whether a peer rotated, so it
// must stop the refresh rather than read as "no token on disk".
func TestRefreshOAuthToken_DoesNotExchangeWithATruncatedConfigFile(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	var exchanges atomic.Int64
	store := newRefreshTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		exchanges.Add(1)
		return freshToken("at1", "rt1"), nil
	})
	require.NoError(t, os.WriteFile(configPath, []byte(`{"providers": {"hyper": {"oauth": {"access_token": "at`), 0o600))

	require.Error(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	require.Zero(t, exchanges.Load())
}

// Disk can hold the very access token the provider just refused, under a
// refresh token a peer rotated. Adopting it would repair nothing, so the
// rotated refresh token on disk is the one spent.
func TestRefreshRejectedOAuthToken_SpendsDiskRefreshTokenWhenDiskHoldsTheRejectedToken(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	held := tokenIssuedAgo(50*time.Minute, time.Hour)
	var spent []string
	store := newStoreHoldingToken(t, configPath, held, func(_ context.Context, _, refreshToken string) (*oauth.Token, error) {
		spent = append(spent, refreshToken)
		return freshToken("at1", "rt1"), nil
	})
	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  held.AccessToken,
		RefreshToken: "rt0-rotated",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})

	require.NoError(t, store.RefreshRejectedOAuthToken(context.Background(), ScopeGlobal, "hyper", held))
	require.Equal(t, []string{"rt0-rotated"}, spent)

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "at1", pc.OAuthToken.AccessToken)
}
