package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// newRefreshTestStore builds a ConfigStore whose hyper provider holds an
// expired OAuth token, persisted both in memory and on disk at configPath.
// Stores that share a configPath also share the per-provider refresh lock,
// which lets a single test process faithfully simulate two crush instances:
// lock.File opens a fresh descriptor per call, so two stores block each
// other on the same lock file exactly as two processes would.
func newRefreshTestStore(t *testing.T, configPath string, exchange func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error)) *ConfigStore {
	t.Helper()

	expired := &oauth.Token{
		AccessToken:  "at0",
		RefreshToken: "rt0",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
	}
	configContent := `{
		"providers": {
			"hyper": {
				"oauth": {
					"access_token": "at0",
					"refresh_token": "rt0",
					"expires_in": 3600,
					"expires_at": ` + fmt.Sprintf("%d", expired.ExpiresAt) + `
				}
			}
		}
	}`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("hyper", ProviderConfig{
		ID:         "hyper",
		Name:       "Hyper",
		APIKey:     expired.AccessToken,
		OAuthToken: expired,
	})

	return &ConfigStore{
		config:         &Config{Providers: providers},
		globalDataPath: configPath,
		workingDir:     filepath.Dir(configPath),
		exchangeToken:  exchange,
	}
}

// TestRefreshOAuthToken_InProcessSingleFlight verifies that a storm of
// concurrent refresh calls for the same provider collapses into a single
// token exchange.
func TestRefreshOAuthToken_InProcessSingleFlight(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "crush.json")

	var exchanges atomic.Int64
	store := newRefreshTestStore(t, configPath, func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error) {
		exchanges.Add(1)
		time.Sleep(50 * time.Millisecond) // hold the flight open so peers join
		return &oauth.Token{
			AccessToken:  "at1",
			RefreshToken: "rt1",
			ExpiresIn:    3600,
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		}, nil
	})

	const goroutines = 20
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, goroutines)
	for range goroutines {
		wg.Go(func() {
			<-start
			errs <- store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper")
		})
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(1), exchanges.Load(), "concurrent refreshes should collapse into one exchange")

	pc, ok := store.config.Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "at1", pc.OAuthToken.AccessToken)
	require.Equal(t, "rt1", pc.OAuthToken.RefreshToken)
}

// TestRefreshOAuthToken_CrossProcessAdopt verifies that when two instances
// share a credential, only one performs the token exchange and the other
// adopts the rotated token from disk rather than reusing the consumed
// refresh token. The fake exchange models a rotating provider: reusing a
// refresh token it has already rotated returns an error, so a second
// exchange would be observable as a failure.
func TestRefreshOAuthToken_CrossProcessAdopt(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "crush.json")

	var (
		mu          sync.Mutex
		current     = "rt0" // the only refresh token the server will accept
		exchanges   atomic.Int64
		reuseErrors atomic.Int64
	)
	exchange := func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error) {
		mu.Lock()
		defer mu.Unlock()
		if refreshToken != current {
			reuseErrors.Add(1)
			return nil, fmt.Errorf("refresh token revoked")
		}
		exchanges.Add(1)
		time.Sleep(50 * time.Millisecond) // hold the lock so the peer must wait
		current = "rt1"
		return &oauth.Token{
			AccessToken:  "at1",
			RefreshToken: "rt1",
			ExpiresIn:    3600,
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		}, nil
	}

	// Two stores sharing the same config file and refresh lock = two
	// "processes".
	a := newRefreshTestStore(t, configPath, exchange)
	b := newRefreshTestStore(t, configPath, exchange)

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, s := range []*ConfigStore{a, b} {
		wg.Go(func() {
			<-start
			errs <- s.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper")
		})
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, int64(1), exchanges.Load(), "only one instance should exchange")
	require.Equal(t, int64(0), reuseErrors.Load(), "no instance should reuse a rotated refresh token")

	// Both instances converge on the rotated token.
	for name, s := range map[string]*ConfigStore{"a": a, "b": b} {
		pc, ok := s.config.Providers.Get("hyper")
		require.True(t, ok, name)
		require.Equal(t, "at1", pc.OAuthToken.AccessToken, name)
		require.Equal(t, "rt1", pc.OAuthToken.RefreshToken, name)
	}
}
