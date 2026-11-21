package config

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// familyProvider is a fake OAuth provider with refresh-token reuse
// detection: every exchange retires the refresh token it was given, and
// presenting a retired one revokes the whole family, after which nothing
// works until the user signs in again.
type familyProvider struct {
	mu           sync.Mutex
	live         string
	retired      map[string]bool
	revoked      bool
	next         int
	exchanges    int
	spentRetired []string
}

func newFamilyProvider(live string) *familyProvider {
	return &familyProvider{live: live, retired: map[string]bool{}, next: 1}
}

func (p *familyProvider) exchange(_ context.Context, _, refreshToken string) (*oauth.Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.retired[refreshToken] {
		p.spentRetired = append(p.spentRetired, refreshToken)
		p.revoked = true
	}
	if p.revoked || refreshToken != p.live {
		return nil, oauth.NewTokenExchangeError(400, `{"error":"invalid_grant"}`)
	}
	p.retired[refreshToken] = true
	p.exchanges++
	p.live = fmt.Sprintf("family-rt%d", p.next)
	token := &oauth.Token{
		AccessToken:  fmt.Sprintf("family-at%d", p.next),
		RefreshToken: p.live,
		// Issued already past its refresh deadline, so every refresh call
		// in the test has to exchange or borrow rather than adopt.
		ExpiresIn: 3600,
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	}
	p.next++
	return token, nil
}

func (p *familyProvider) requireIntact(t *testing.T) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	require.Empty(t, p.spentRetired, "a retired refresh token was spent")
	require.False(t, p.revoked, "the token family was revoked")
}

// TestRefreshOAuthToken_TwoSessionsNeverSpendARetiredToken_Sequential has
// two sessions sharing one config file take turns refreshing. Each one's
// in-memory refresh token is retired by the other's exchange, so each
// refresh must spend the token on disk instead.
func TestRefreshOAuthToken_TwoSessionsNeverSpendARetiredToken_Sequential(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	provider := newFamilyProvider("rt0")
	a := newRefreshTestStore(t, configPath, provider.exchange)
	b := newRefreshTestStore(t, configPath, provider.exchange)

	const rounds = 10
	for i := range rounds {
		s := a
		if i%2 == 1 {
			s = b
		}
		require.NoError(t, s.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"), "round %d", i)
	}

	provider.requireIntact(t)
	require.Equal(t, rounds, provider.exchanges)
}

// TestRefreshOAuthToken_TwoSessionsNeverSpendARetiredToken_Concurrent runs
// the same two sessions refreshing at once from many goroutines each.
func TestRefreshOAuthToken_TwoSessionsNeverSpendARetiredToken_Concurrent(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	provider := newFamilyProvider("rt0")
	a := newRefreshTestStore(t, configPath, provider.exchange)
	b := newRefreshTestStore(t, configPath, provider.exchange)

	const (
		goroutines = 8
		rounds     = 5
	)
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, 2*goroutines*rounds)
	for _, s := range []*ConfigStore{a, b} {
		for range goroutines {
			wg.Go(func() {
				<-start
				for range rounds {
					errs <- s.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper")
				}
			})
		}
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	provider.requireIntact(t)
	require.Positive(t, provider.exchanges)
}
