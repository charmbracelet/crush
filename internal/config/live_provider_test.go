package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

type mockLiveProviderClient struct {
	provider  catwalk.Provider
	err       error
	callCount int
}

func (m *mockLiveProviderClient) Get(ctx context.Context) (catwalk.Provider, error) {
	m.callCount++
	return m.provider, m.err
}

func TestLiveProviderSync_GetPanicIfNotInit(t *testing.T) {
	t.Parallel()

	syncer := &liveProviderSync{}
	require.Panics(t, func() {
		_, _ = syncer.Get(t.Context())
	})
}

func TestLiveProviderSync_GetAutoUpdateDisabledReturnsSeed(t *testing.T) {
	t.Parallel()

	seed := testLiveSeedProvider()
	client := &mockLiveProviderClient{
		provider: catwalk.Provider{Models: []catwalk.Model{{ID: "live-model"}}},
	}
	syncer := &liveProviderSync{}
	syncer.Init(client, t.TempDir()+"/provider.json", false, seed, true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, seed, provider)
	require.Equal(t, 0, client.callCount)
}

func TestLiveProviderSync_GetWithoutCredentialsReturnsSeed(t *testing.T) {
	t.Parallel()

	seed := testLiveSeedProvider()
	client := &mockLiveProviderClient{
		provider: catwalk.Provider{Models: []catwalk.Model{{ID: "live-model"}}},
	}
	syncer := &liveProviderSync{}
	syncer.Init(client, t.TempDir()+"/provider.json", true, seed, false)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, seed, provider)
	require.Equal(t, 0, client.callCount)
}

func TestLiveProviderSync_GetWarmCacheReturnsCachedWithoutFetch(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/provider.json"
	cached := testLiveSeedProvider()
	cached.Name = "Cached Provider"
	cached.Models = []catwalk.Model{{ID: "cached-model", Name: "Cached Model"}}
	writeLiveProviderCache(t, path, cached)

	client := &mockLiveProviderClient{
		provider: catwalk.Provider{Models: []catwalk.Model{{ID: "live-model"}}},
	}
	syncer := &liveProviderSync{}
	syncer.Init(client, path, true, testLiveSeedProvider(), true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, cached, provider)
	require.Equal(t, 0, client.callCount)
}

func TestLiveProviderSync_GetStaleCacheFetchesMergesAndStores(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/provider.json"
	cached := testLiveSeedProvider()
	cached.Models = []catwalk.Model{{ID: "cached-model", Name: "Cached Model"}}
	writeLiveProviderCache(t, path, cached)
	staleTime := time.Now().Add(-2 * liveModelsTTL)
	require.NoError(t, os.Chtimes(path, staleTime, staleTime))

	seed := testLiveSeedProvider()
	client := &mockLiveProviderClient{
		provider: catwalk.Provider{
			Models: []catwalk.Model{
				{ID: "live-model", Name: "Live Model"},
				{ID: "other-live-model", Name: "Other Live Model"},
			},
		},
	}
	syncer := &liveProviderSync{}
	syncer.Init(client, path, true, seed, true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, client.callCount)
	require.Equal(t, seed.ID, provider.ID)
	require.Equal(t, seed.Name, provider.Name)
	require.Equal(t, seed.APIEndpoint, provider.APIEndpoint)
	require.Equal(t, seed.Type, provider.Type)
	require.Equal(t, "live-model", provider.DefaultLargeModelID)
	require.Equal(t, "live-model", provider.DefaultSmallModelID)
	require.Equal(t, []catwalk.Model{
		{ID: "live-model", Name: "Live Model"},
		{ID: "other-live-model", Name: "Other Live Model"},
	}, provider.Models)

	stored, _, err := newCache[catwalk.Provider](path).Get()
	require.NoError(t, err)
	require.Equal(t, provider, stored)
}

func TestLiveProviderSync_GetStoreFailureStillReturnsMerged(t *testing.T) {
	t.Parallel()

	// Point the cache at a path whose parent is a regular file so that
	// Store fails; the merged provider must still be returned without
	// error.
	blocker := t.TempDir() + "/blocker"
	require.NoError(t, os.WriteFile(blocker, []byte("not a dir"), 0o644))
	path := blocker + "/provider.json"

	seed := testLiveSeedProvider()
	client := &mockLiveProviderClient{
		provider: catwalk.Provider{Models: []catwalk.Model{{ID: "live-model", Name: "Live Model"}}},
	}
	syncer := &liveProviderSync{}
	syncer.Init(client, path, true, seed, true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, client.callCount)
	require.Equal(t, []catwalk.Model{{ID: "live-model", Name: "Live Model"}}, provider.Models)
}

func TestLiveProviderSync_GetFetchErrorUsesCached(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/provider.json"
	cached := testLiveSeedProvider()
	cached.Name = "Cached Provider"
	cached.Models = []catwalk.Model{{ID: "cached-model", Name: "Cached Model"}}
	writeLiveProviderCache(t, path, cached)
	staleTime := time.Now().Add(-2 * liveModelsTTL)
	require.NoError(t, os.Chtimes(path, staleTime, staleTime))

	client := &mockLiveProviderClient{err: errors.New("network error")}
	syncer := &liveProviderSync{}
	syncer.Init(client, path, true, testLiveSeedProvider(), true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, cached, provider)
	require.Equal(t, 1, client.callCount)
}

func TestLiveProviderSync_GetDeadlineExceededUsesCached(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/provider.json"
	cached := testLiveSeedProvider()
	cached.Name = "Cached Provider"
	cached.Models = []catwalk.Model{{ID: "cached-model", Name: "Cached Model"}}
	writeLiveProviderCache(t, path, cached)
	staleTime := time.Now().Add(-2 * liveModelsTTL)
	require.NoError(t, os.Chtimes(path, staleTime, staleTime))

	client := &mockLiveProviderClient{err: context.DeadlineExceeded}
	syncer := &liveProviderSync{}
	syncer.Init(client, path, true, testLiveSeedProvider(), true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, cached, provider)
	require.Equal(t, 1, client.callCount)
}

func TestLiveProviderSync_GetFetchErrorUsesSeedWithoutCache(t *testing.T) {
	t.Parallel()

	seed := testLiveSeedProvider()
	client := &mockLiveProviderClient{err: errors.New("network error")}
	syncer := &liveProviderSync{}
	syncer.Init(client, t.TempDir()+"/provider.json", true, seed, true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, seed, provider)
	require.Equal(t, 1, client.callCount)
}

func TestLiveProviderSync_GetEmptyModelsUsesFallback(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/provider.json"
	cached := testLiveSeedProvider()
	cached.Name = "Cached Provider"
	cached.Models = []catwalk.Model{{ID: "cached-model", Name: "Cached Model"}}
	writeLiveProviderCache(t, path, cached)
	staleTime := time.Now().Add(-2 * liveModelsTTL)
	require.NoError(t, os.Chtimes(path, staleTime, staleTime))

	client := &mockLiveProviderClient{provider: catwalk.Provider{ID: "test-live"}}
	syncer := &liveProviderSync{}
	syncer.Init(client, path, true, testLiveSeedProvider(), true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, cached, provider)
	require.Equal(t, 1, client.callCount)
}

func TestLiveProviderSync_GetCalledMultipleTimesUsesOnce(t *testing.T) {
	t.Parallel()

	client := &mockLiveProviderClient{
		provider: catwalk.Provider{Models: []catwalk.Model{{ID: "live-model", Name: "Live Model"}}},
	}
	syncer := &liveProviderSync{}
	syncer.Init(client, t.TempDir()+"/provider.json", true, testLiveSeedProvider(), true)

	provider1, err1 := syncer.Get(t.Context())
	require.NoError(t, err1)
	provider2, err2 := syncer.Get(t.Context())
	require.NoError(t, err2)
	require.Equal(t, provider1, provider2)
	require.Equal(t, 1, client.callCount)
}

func TestCacheAge(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/provider.json"
	_, ok := cacheAge(path)
	require.False(t, ok)

	writeLiveProviderCache(t, path, testLiveSeedProvider())
	age, ok := cacheAge(path)
	require.True(t, ok)
	require.Less(t, age, liveModelsTTL)

	future := time.Now().Add(liveModelsTTL)
	require.NoError(t, os.Chtimes(path, future, future))
	age, ok = cacheAge(path)
	require.True(t, ok)
	require.Zero(t, age)
}

func testLiveSeedProvider() catwalk.Provider {
	return catwalk.Provider{
		ID:                  "test-live",
		Name:                "Test Live",
		APIEndpoint:         "https://example.com/v1",
		Type:                catwalk.TypeOpenAICompat,
		DefaultLargeModelID: "seed-large-model",
		DefaultSmallModelID: "seed-small-model",
		DefaultHeaders: map[string]string{
			"X-Test": "seed",
		},
		Models: []catwalk.Model{
			{ID: "seed-large-model", Name: "Seed Large Model"},
			{ID: "seed-small-model", Name: "Seed Small Model"},
		},
	}
}

func writeLiveProviderCache(t *testing.T, path string, provider catwalk.Provider) {
	t.Helper()

	data, err := json.Marshal(provider)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
}
