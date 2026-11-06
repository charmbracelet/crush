package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// creditsRecorder is a fake credits fetcher that records the API keys it
// was called with. When release is non-nil, fetches block until it is
// closed, which lets tests hold one fetch in flight.
type creditsRecorder struct {
	release chan struct{}

	mu    sync.Mutex
	keys  []string
	calls int
}

func (r *creditsRecorder) fetch(ctx context.Context, apiKey string) (*int, error) {
	r.mu.Lock()
	r.keys = append(r.keys, apiKey)
	r.calls++
	release := r.release
	r.mu.Unlock()

	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
		}
	}

	balance := 7
	return &balance, nil
}

func (r *creditsRecorder) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *creditsRecorder) apiKeys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.keys...)
}

func newTestHyperCreditsModel(rec *creditsRecorder, apiKey func() string) *hyperCreditsModel {
	return &hyperCreditsModel{
		LanguageModel: &fakeLanguageModel{},
		apiKey:        apiKey,
		fetch:         rec.fetch,
	}
}

func TestNewHyperCreditsModel_NoModelOrResolver(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	require.Nil(t, newHyperCreditsModel(nil, func() string { return "key" }))
	require.Same(t, inner, newHyperCreditsModel(inner, nil))
}

func TestHyperCreditsModel_GenerateRefreshesBalance(t *testing.T) {
	t.Parallel()

	rec := &creditsRecorder{}
	m := newTestHyperCreditsModel(rec, func() string { return "test-key" })

	_, err := m.Generate(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	require.Eventually(t, func() bool { return rec.callCount() == 1 }, 5*time.Second, 5*time.Millisecond)
	require.Equal(t, []string{"test-key"}, rec.apiKeys())
}

func TestHyperCreditsModel_StreamRefreshesOnlyOnceConsumed(t *testing.T) {
	t.Parallel()

	rec := &creditsRecorder{}
	m := newTestHyperCreditsModel(rec, func() string { return "test-key" })
	inner := m.LanguageModel.(*fakeLanguageModel)
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{})
		yield(fantasy.StreamPart{})
	}

	stream, err := m.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	require.Zero(t, rec.callCount(), "the balance must not be refreshed before the stream is consumed")

	parts := 0
	for range stream {
		parts++
	}
	require.Equal(t, 2, parts)

	require.Eventually(t, func() bool { return rec.callCount() == 1 }, 5*time.Second, 5*time.Millisecond)
}

func TestHyperCreditsModel_SkipsFetchWithoutAPIKey(t *testing.T) {
	t.Parallel()

	rec := &creditsRecorder{}
	m := newTestHyperCreditsModel(rec, func() string { return "" })

	_, err := m.Generate(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	require.Zero(t, rec.callCount())
}

func TestHyperCreditsModel_CoalescesRefreshes(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	rec := &creditsRecorder{release: release}
	m := newTestHyperCreditsModel(rec, func() string { return "test-key" })

	// Hold the first fetch in flight, then queue two more refreshes.
	m.refreshBalance()
	require.Eventually(t, func() bool { return rec.callCount() == 1 }, 5*time.Second, 5*time.Millisecond)
	m.refreshBalance()
	m.refreshBalance()

	close(release)

	// The queued refreshes coalesce into a single follow-up fetch.
	require.Eventually(t, func() bool { return rec.callCount() == 2 }, 5*time.Second, 5*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 2, rec.callCount(), "coalesced refreshes must not run one fetch each")
}
