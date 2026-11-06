package config

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// A background poll that refreshes on a timer used to send the same dead
// refresh token to the provider once a minute for hours. Once refused, it
// must keep failing as revoked without another round trip.
func TestRefreshOAuthToken_RefusedRefreshTokenIsNotRetried(t *testing.T) {
	isolateHyperCredentials(t)
	configPath := filepath.Join(t.TempDir(), "crush.json")

	var exchanges atomic.Int64
	store := newRefreshTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		exchanges.Add(1)
		return nil, &oauth.TokenExchangeError{StatusCode: 401, Body: `{"error":"refresh token revoked"}`}
	})

	for range 3 {
		err := store.RefreshOAuthToken(t.Context(), ScopeGlobal, "hyper")
		var exchangeErr *oauth.TokenExchangeError
		require.True(t, errors.As(err, &exchangeErr))
		require.True(t, exchangeErr.IsRefreshTokenRevoked())
	}
	require.Equal(t, int64(1), exchanges.Load())
}

// A failure that is not a refusal, such as an outage, says nothing about the
// credential, so the next refresh must try again.
func TestRefreshOAuthToken_TransientFailureIsRetried(t *testing.T) {
	isolateHyperCredentials(t)
	configPath := filepath.Join(t.TempDir(), "crush.json")

	var exchanges atomic.Int64
	store := newRefreshTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		exchanges.Add(1)
		return nil, &oauth.TokenExchangeError{StatusCode: 503, Body: `unavailable`}
	})

	for range 2 {
		require.Error(t, store.RefreshOAuthToken(t.Context(), ScopeGlobal, "hyper"))
	}
	require.Equal(t, int64(2), exchanges.Load())
}

// A refusal could be a provider's mistake, so it is only trusted for a few
// minutes before the token is offered again.
func TestRefreshOAuthToken_RefusalExpires(t *testing.T) {
	isolateHyperCredentials(t)
	configPath := filepath.Join(t.TempDir(), "crush.json")

	var exchanges atomic.Int64
	store := newRefreshTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		exchanges.Add(1)
		return nil, &oauth.TokenExchangeError{StatusCode: 401, Body: `{"error":"refresh token revoked"}`}
	})
	clock := time.Now()
	store.now = func() time.Time { return clock }

	require.Error(t, store.RefreshOAuthToken(t.Context(), ScopeGlobal, "hyper"))
	clock = clock.Add(rejectedRefreshTokenTTL - time.Second)
	require.Error(t, store.RefreshOAuthToken(t.Context(), ScopeGlobal, "hyper"))
	require.Equal(t, int64(1), exchanges.Load())

	clock = clock.Add(time.Second)
	require.Error(t, store.RefreshOAuthToken(t.Context(), ScopeGlobal, "hyper"))
	require.Equal(t, int64(2), exchanges.Load())
}
