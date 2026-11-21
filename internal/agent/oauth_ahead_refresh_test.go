package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func aheadRefreshProvider(id string) config.ProviderConfig {
	return config.ProviderConfig{
		ID: id,
		OAuthToken: &oauth.Token{
			AccessToken:  "at",
			RefreshToken: "rt",
			ExpiresIn:    3600,
			ExpiresAt:    time.Now().Add(10 * time.Minute).Unix(),
		},
	}
}

// A burst of turns inside the ahead-of-time window starts one renewal per
// provider, not one goroutine per turn.
func TestAheadRefreshRunsOncePerProvider(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	var calls atomic.Int64
	c := &coordinator{refreshAhead: func(ctx context.Context, providerID string) error {
		calls.Add(1)
		<-release
		return errors.New("stop before rebuilding")
	}}

	for range 20 {
		c.refreshOAuth2TokenAhead(t.Context(), aheadRefreshProvider("hyper"))
	}
	c.refreshOAuth2TokenAhead(t.Context(), aheadRefreshProvider("copilot"))
	close(release)
	c.stopBackgroundRefreshes(5 * time.Second)

	require.Equal(t, int64(2), calls.Load(), "one renewal for each provider")
}

// Shutdown waits for a renewal in flight, since exiting before its result
// is saved strands the account, and starts no new ones afterwards.
func TestShutdownWaitsForAheadRefreshAndStopsNewOnes(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	var finished atomic.Bool
	var calls atomic.Int64
	c := &coordinator{refreshAhead: func(ctx context.Context, providerID string) error {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			finished.Store(true)
		}
		return errors.New("stop before rebuilding")
	}}

	c.refreshOAuth2TokenAhead(t.Context(), aheadRefreshProvider("hyper"))
	<-started

	stopped := make(chan struct{})
	go func() {
		c.stopBackgroundRefreshes(5 * time.Second)
		close(stopped)
	}()
	select {
	case <-stopped:
		t.Fatal("shutdown did not wait for the renewal in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-stopped
	require.True(t, finished.Load())

	c.refreshOAuth2TokenAhead(t.Context(), aheadRefreshProvider("copilot"))
	c.stopBackgroundRefreshes(time.Second)
	require.Equal(t, int64(1), calls.Load(), "no renewal starts once shutdown began")
}

// The wait is bounded, so a wedged exchange cannot hang shutdown.
func TestShutdownWaitForAheadRefreshIsBounded(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	c := &coordinator{refreshAhead: func(ctx context.Context, providerID string) error {
		<-release
		return errors.New("stop before rebuilding")
	}}
	c.refreshOAuth2TokenAhead(t.Context(), aheadRefreshProvider("hyper"))

	begun := time.Now()
	c.stopBackgroundRefreshes(50 * time.Millisecond)
	require.Less(t, time.Since(begun), 2*time.Second)
}

// A renewal that has not reached the provider yet (still waiting for the
// refresh lock, say) is cancelled when shutdown begins, so shutdown does
// not sit out the whole wait budget for it.
func TestShutdownCancelsAheadRefreshNotYetExchanging(t *testing.T) {
	t.Parallel()

	waiting := make(chan struct{})
	c := &coordinator{refreshAhead: func(ctx context.Context, providerID string) error {
		close(waiting)
		<-ctx.Done()
		return ctx.Err()
	}}
	c.refreshOAuth2TokenAhead(t.Context(), aheadRefreshProvider("hyper"))
	<-waiting

	begun := time.Now()
	c.stopBackgroundRefreshes(30 * time.Second)
	require.Less(t, time.Since(begun), 5*time.Second)
}
