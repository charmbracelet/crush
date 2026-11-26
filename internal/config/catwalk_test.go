package config

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	xetag "github.com/charmbracelet/x/etag"
	"github.com/stretchr/testify/require"
)

type mockCatwalkClient struct {
	providers []catwalk.Provider
	err       error
	callCount int
}

func (m *mockCatwalkClient) GetProviders(ctx context.Context, etag string) ([]catwalk.Provider, []byte, error) {
	m.callCount++
	if m.err != nil {
		return nil, nil, m.err
	}
	raw, err := json.Marshal(m.providers)
	return m.providers, raw, err
}

// TestCatwalkSync_CachedETagMatchesServer guards against caching a
// re-marshaled catalog: fields this build does not know would be dropped,
// the cached ETag would never match the server's, and every startup would
// download the whole catalog again.
func TestCatwalkSync_CachedETagMatchesServer(t *testing.T) {
	t.Parallel()

	body := []byte(`[{"id":"fresh","name":"Fresh","field_from_the_future":{"x":1}}]`)
	tag := xetag.Of(body)
	var full, notModified atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Trim(r.Header.Get("If-None-Match"), `"`) == tag {
			notModified.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		full.Add(1)
		w.Header().Set("ETag", `"`+tag+`"`)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	path := t.TempDir() + "/providers.json"

	first := newCatwalkSync()
	first.Init(newCatwalkClient(server.URL).GetProviders, path, true)
	providers, err := first.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Fresh", providers[0].Name)
	changed, err := first.Revalidate(t.Context())
	require.NoError(t, err)
	require.True(t, changed, "a cold start's fetch is reported as a change")

	second := newCatwalkSync()
	second.Init(newCatwalkClient(server.URL).GetProviders, path, true)
	providers, err = second.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Fresh", providers[0].Name)
	require.EqualValues(t, 0, notModified.Load(), "a startup with a cache does not wait on the network")
	changed, err = second.Revalidate(t.Context())
	require.NoError(t, err)
	require.False(t, changed, "an unchanged catalog is not a change")
	require.EqualValues(t, 1, full.Load(), "the second startup must not download the catalog again")
	require.EqualValues(t, 1, notModified.Load())
}

func TestCatwalkSync_GetPanicIfNotInit(t *testing.T) {
	t.Parallel()

	syncer := newCatwalkSync()
	require.Panics(t, func() {
		_, _ = syncer.Get(t.Context())
	})
}

func TestCatwalkSync_GetWithAutoUpdateDisabled(t *testing.T) {
	t.Parallel()

	syncer := newCatwalkSync()
	client := &mockCatwalkClient{
		providers: []catwalk.Provider{{Name: "should-not-be-used"}},
	}
	path := t.TempDir() + "/providers.json"

	syncer.Init(client.GetProviders, path, false)

	providers, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, providers)
	require.Equal(t, 0, client.callCount, "Client should not be called when autoupdate is disabled")

	// No cache file: fall back to embedded providers.
	for _, p := range providers {
		require.NotEqual(t, "should-not-be-used", p.Name)
	}
}

func TestCatwalkSync_GetWithAutoUpdateDisabledUsesCache(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/providers.json"

	cachedProviders := []catwalk.Provider{
		{Name: "Cached Manual Update", ID: "cached-manual"},
	}
	data, err := json.Marshal(cachedProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	syncer := newCatwalkSync()
	client := &mockCatwalkClient{
		providers: []catwalk.Provider{{Name: "should-not-be-used"}},
	}

	syncer.Init(client.GetProviders, path, false)

	providers, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Len(t, providers, 1)
	require.Equal(t, "Cached Manual Update", providers[0].Name)
	require.Equal(t, 0, client.callCount, "Client should not be called when autoupdate is disabled")
}

func TestCatwalkSync_GetFreshProviders(t *testing.T) {
	t.Parallel()

	syncer := newCatwalkSync()
	client := &mockCatwalkClient{
		providers: []catwalk.Provider{
			{Name: "Fresh Provider", ID: "fresh"},
		},
	}
	path := t.TempDir() + "/providers.json"

	syncer.Init(client.GetProviders, path, true)

	providers, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Len(t, providers, 1)
	require.Equal(t, "Fresh Provider", providers[0].Name)
	require.Equal(t, 1, client.callCount)

	// Verify cache was written.
	fileInfo, err := os.Stat(path)
	require.NoError(t, err)
	require.False(t, fileInfo.IsDir())
}

func TestCatwalkSync_GetNotModifiedUsesCached(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/providers.json"

	// Create cache file.
	cachedProviders := []catwalk.Provider{
		{Name: "Cached Provider", ID: "cached"},
	}
	data, err := json.Marshal(cachedProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	syncer := newCatwalkSync()
	client := &mockCatwalkClient{
		err: catwalk.ErrNotModified,
	}

	syncer.Init(client.GetProviders, path, true)

	providers, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Len(t, providers, 1)
	require.Equal(t, "Cached Provider", providers[0].Name)
	require.Equal(t, 0, client.callCount, "a cached catalog is served without a request")

	changed, err := syncer.Revalidate(t.Context())
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, 1, client.callCount)
	changed, err = syncer.Revalidate(t.Context())
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, 1, client.callCount, "a catalog is revalidated once per process")
	providers, _ = syncer.Get(t.Context())
	require.Equal(t, "Cached Provider", providers[0].Name)
}

func TestCatwalkSync_RevalidateReplacesCachedCatalog(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/providers.json"
	data, err := json.Marshal([]catwalk.Provider{{Name: "Cached Provider", ID: "cached"}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	syncer := newCatwalkSync()
	client := &mockCatwalkClient{providers: []catwalk.Provider{{Name: "Fresh Provider", ID: "fresh"}}}
	syncer.Init(client.GetProviders, path, true)

	providers, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Cached Provider", providers[0].Name)

	changed, err := syncer.Revalidate(t.Context())
	require.NoError(t, err)
	require.True(t, changed)
	providers, _ = syncer.Get(t.Context())
	require.Equal(t, "Fresh Provider", providers[0].Name)

	cached, _, err := syncer.cache.Get()
	require.NoError(t, err)
	require.Equal(t, "Fresh Provider", cached[0].Name, "the next startup gets the fresh catalog")
}

func TestCatwalkSync_GetEmptyResultFallbackToCached(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/providers.json"

	// Create cache file.
	cachedProviders := []catwalk.Provider{
		{Name: "Cached Provider", ID: "cached"},
	}
	data, err := json.Marshal(cachedProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	syncer := newCatwalkSync()
	client := &mockCatwalkClient{
		providers: []catwalk.Provider{}, // Empty result.
	}

	syncer.Init(client.GetProviders, path, true)

	_, err = syncer.Get(t.Context())
	require.NoError(t, err)
	changed, err := syncer.Revalidate(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty Catwalk providers")
	require.False(t, changed)
	providers, _ := syncer.Get(t.Context())
	require.Len(t, providers, 1)
	require.Equal(t, "Cached Provider", providers[0].Name)
}

func TestCatwalkSync_GetEmptyCacheDefaultsToEmbedded(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/providers.json"

	// Create empty cache file.
	emptyProviders := []catwalk.Provider{}
	data, err := json.Marshal(emptyProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	syncer := newCatwalkSync()
	client := &mockCatwalkClient{
		err: errors.New("network error"),
	}

	syncer.Init(client.GetProviders, path, true)

	providers, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, providers, "Should fall back to embedded providers")

	// Verify it's embedded providers by checking we have multiple common ones.
	require.Greater(t, len(providers), 5)
}

func TestCatwalkSync_GetClientError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/providers.json"

	syncer := newCatwalkSync()
	client := &mockCatwalkClient{
		err: errors.New("network error"),
	}

	syncer.Init(client.GetProviders, path, true)

	providers, err := syncer.Get(t.Context())
	require.NoError(t, err) // Should fall back to embedded.
	require.NotEmpty(t, providers)
}

func TestCatwalkSync_GetCalledMultipleTimesUsesOnce(t *testing.T) {
	t.Parallel()

	syncer := newCatwalkSync()
	client := &mockCatwalkClient{
		providers: []catwalk.Provider{
			{Name: "Provider", ID: "test"},
		},
	}
	path := t.TempDir() + "/providers.json"

	syncer.Init(client.GetProviders, path, true)

	// Call Get multiple times.
	providers1, err1 := syncer.Get(t.Context())
	require.NoError(t, err1)
	require.Len(t, providers1, 1)

	providers2, err2 := syncer.Get(t.Context())
	require.NoError(t, err2)
	require.Len(t, providers2, 1)

	// Client should only be called once due to sync.Once.
	require.Equal(t, 1, client.callCount)
}

// TestSyncer_ReportsEachChangeOnce guards against reacting to one catalog
// update more than once, e.g. per workspace in server mode.
func TestSyncer_ReportsEachChangeOnce(t *testing.T) {
	t.Parallel()

	fresh := []catwalk.Provider{{Name: "Fresh Provider", ID: "fresh"}}
	revalidate := func(t *testing.T, s *syncer[[]catwalk.Provider]) bool {
		t.Helper()
		changed, err := s.Revalidate(t.Context())
		require.NoError(t, err)
		return changed
	}

	t.Run("a cold start's fetch is reported once", func(t *testing.T) {
		t.Parallel()
		s := newCatwalkSync()
		s.Init((&mockCatwalkClient{providers: fresh}).GetProviders, t.TempDir()+"/providers.json", true)
		_, err := s.Get(t.Context())
		require.NoError(t, err)
		require.True(t, revalidate(t, s))
		require.False(t, revalidate(t, s))
	})

	t.Run("not modified is no change", func(t *testing.T) {
		t.Parallel()
		path := t.TempDir() + "/providers.json"
		require.NoError(t, os.WriteFile(path, []byte(`[{"name":"Cached","id":"cached"}]`), 0o644))
		s := newCatwalkSync()
		s.Init((&mockCatwalkClient{err: catwalk.ErrNotModified}).GetProviders, path, true)
		_, err := s.Get(t.Context())
		require.NoError(t, err)
		require.False(t, revalidate(t, s))
	})

	t.Run("a network error is no change", func(t *testing.T) {
		t.Parallel()
		s := newCatwalkSync()
		s.Init((&mockCatwalkClient{err: errors.New("network error")}).GetProviders, t.TempDir()+"/providers.json", true)
		_, err := s.Get(t.Context())
		require.NoError(t, err)
		require.False(t, revalidate(t, s))
	})

	t.Run("auto update disabled never changes", func(t *testing.T) {
		t.Parallel()
		s := newCatwalkSync()
		s.Init((&mockCatwalkClient{providers: fresh}).GetProviders, t.TempDir()+"/providers.json", false)
		_, err := s.Get(t.Context())
		require.NoError(t, err)
		require.False(t, revalidate(t, s))
	})
}
