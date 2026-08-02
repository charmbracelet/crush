package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func resetProviderState() {
	providerOnce = sync.Once{}
	providerList = nil
	providerErr = nil
	catwalkSyncer = &catwalkSync{}
	hyperSyncer = &hyperSync{}
}

func TestProviders_Integration_AutoUpdateDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Use a test-specific instance to avoid global state interference.
	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	originalCatwalSyncer := catwalkSyncer
	originalHyperSyncer := hyperSyncer
	defer func() {
		catwalkSyncer = originalCatwalSyncer
		hyperSyncer = originalHyperSyncer
	}()

	catwalkSyncer = testCatwalkSyncer
	hyperSyncer = testHyperSyncer

	resetProviderState()
	defer resetProviderState()

	cfg := &Config{
		Options: &Options{
			DisableProviderAutoUpdate: true,
		},
	}

	providers, err := Providers(cfg)
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.Greater(t, len(providers), 5, "Expected embedded providers")
}

func TestProviders_Integration_WithMockClients(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create fresh syncers for this test.
	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	// Initialize with mock clients.
	mockCatwalkClient := &mockCatwalkClient{
		providers: []catwalk.Provider{
			{Name: "Provider1", ID: "p1"},
			{Name: "Provider2", ID: "p2"},
		},
	}
	mockHyperClient := &mockHyperClient{
		provider: catwalk.Provider{
			Name: "Hyper",
			ID:   "hyper",
			Models: []catwalk.Model{
				{ID: "hyper-1", Name: "Hyper Model"},
			},
		},
	}

	catwalkPath := tmpDir + "/crush/providers.json"
	hyperPath := tmpDir + "/crush/hyper.json"

	testCatwalkSyncer.Init(mockCatwalkClient, catwalkPath, true)
	testHyperSyncer.Init(mockHyperClient, hyperPath, true)

	// Get providers from each syncer.
	catwalkProviders, err := testCatwalkSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Len(t, catwalkProviders, 2)

	hyperProvider, err := testHyperSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Hyper", hyperProvider.Name)

	// Verify total.
	allProviders := append(catwalkProviders, hyperProvider)
	require.Len(t, allProviders, 3)
}

func TestProviders_Integration_WithCachedData(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create cache files.
	catwalkPath := tmpDir + "/crush/providers.json"
	hyperPath := tmpDir + "/crush/hyper.json"

	require.NoError(t, os.MkdirAll(tmpDir+"/crush", 0o755))

	// Write Catwalk cache.
	catwalkProviders := []catwalk.Provider{
		{Name: "Cached1", ID: "c1"},
		{Name: "Cached2", ID: "c2"},
	}
	data, err := json.Marshal(catwalkProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(catwalkPath, data, 0o644))

	// Write Hyper cache.
	hyperProvider := catwalk.Provider{
		Name: "Cached Hyper",
		ID:   "hyper",
	}
	data, err = json.Marshal(hyperProvider)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(hyperPath, data, 0o644))

	// Create fresh syncers.
	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	// Mock clients that return ErrNotModified.
	mockCatwalkClient := &mockCatwalkClient{
		err: catwalk.ErrNotModified,
	}
	mockHyperClient := &mockHyperClient{
		err: catwalk.ErrNotModified,
	}

	testCatwalkSyncer.Init(mockCatwalkClient, catwalkPath, true)
	testHyperSyncer.Init(mockHyperClient, hyperPath, true)

	// Get providers - should use cached.
	catwalkResult, err := testCatwalkSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Len(t, catwalkResult, 2)
	require.Equal(t, "Cached1", catwalkResult[0].Name)

	hyperResult, err := testHyperSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Cached Hyper", hyperResult.Name)
}

func TestProviders_Integration_CatwalkFailsHyperSucceeds(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	// Catwalk fails, Hyper succeeds.
	mockCatwalkClient := &mockCatwalkClient{
		err: catwalk.ErrNotModified, // Will use embedded.
	}
	mockHyperClient := &mockHyperClient{
		provider: catwalk.Provider{
			Name: "Hyper",
			ID:   "hyper",
			Models: []catwalk.Model{
				{ID: "hyper-1", Name: "Hyper Model"},
			},
		},
	}

	catwalkPath := tmpDir + "/crush/providers.json"
	hyperPath := tmpDir + "/crush/hyper.json"

	testCatwalkSyncer.Init(mockCatwalkClient, catwalkPath, true)
	testHyperSyncer.Init(mockHyperClient, hyperPath, true)

	catwalkResult, err := testCatwalkSyncer.Get(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, catwalkResult) // Should have embedded.

	hyperResult, err := testHyperSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Hyper", hyperResult.Name)
}

func TestProviders_Integration_BothFail(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	testCatwalkSyncer := &catwalkSync{}
	testHyperSyncer := &hyperSync{}

	// Both fail.
	mockCatwalkClient := &mockCatwalkClient{
		err: catwalk.ErrNotModified,
	}
	mockHyperClient := &mockHyperClient{
		provider: catwalk.Provider{}, // Empty provider.
	}

	catwalkPath := tmpDir + "/crush/providers.json"
	hyperPath := tmpDir + "/crush/hyper.json"

	testCatwalkSyncer.Init(mockCatwalkClient, catwalkPath, true)
	testHyperSyncer.Init(mockHyperClient, hyperPath, true)

	catwalkResult, err := testCatwalkSyncer.Get(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, catwalkResult) // Should fall back to embedded.

	hyperResult, err := testHyperSyncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Charm Hyper", hyperResult.Name) // Falls back to embedded when no models.
}

func TestCache_StoreAndGet(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := tmpDir + "/test.json"

	cache := newCache[[]catwalk.Provider](cachePath)

	providers := []catwalk.Provider{
		{Name: "Provider1", ID: "p1"},
		{Name: "Provider2", ID: "p2"},
	}

	// Store.
	err := cache.Store(providers)
	require.NoError(t, err)

	// Get.
	result, etag, err := cache.Get()
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "Provider1", result[0].Name)
	require.NotEmpty(t, etag)
}

func TestCache_GetNonExistent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := tmpDir + "/nonexistent.json"

	cache := newCache[[]catwalk.Provider](cachePath)

	_, _, err := cache.Get()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read provider cache file")
}

func TestCache_GetInvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := tmpDir + "/invalid.json"

	require.NoError(t, os.WriteFile(cachePath, []byte("invalid json"), 0o644))

	cache := newCache[[]catwalk.Provider](cachePath)

	_, _, err := cache.Get()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal provider data from cache")
}

func TestCachePathFor(t *testing.T) {
	tests := []struct {
		name        string
		xdgDataHome string
		expected    string
	}{
		{
			name:        "with XDG_DATA_HOME",
			xdgDataHome: "/custom/data",
			expected:    "/custom/data/crush/providers.json",
		},
		{
			name:        "without XDG_DATA_HOME",
			xdgDataHome: "",
			expected:    "", // Will use platform-specific default.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.xdgDataHome != "" {
				t.Setenv("XDG_DATA_HOME", tt.xdgDataHome)
			} else {
				t.Setenv("XDG_DATA_HOME", "")
			}

			result := cachePathFor("providers")
			if tt.expected != "" {
				require.Equal(t, tt.expected, filepath.ToSlash(result))
			} else {
				require.Contains(t, result, "crush")
				require.Contains(t, result, "providers.json")
			}
		})
	}
}

// TestProviders_KeepsCatalogWhenCachingFails covers the case that used to
// sign Hyper users out: the provider list was fetched successfully but could
// not be written to the on-disk cache, and Providers discarded it. Hyper's
// endpoint and models live in the catalog rather than in the user's config,
// so losing it there removed the provider entirely and invalidated the
// user's saved model.
func TestProviders_KeepsCatalogWhenCachingFails(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// A file where a directory needs to be, so every cache write fails.
	blocked := filepath.Join(tmpDir, "blocked")
	require.NoError(t, os.WriteFile(blocked, []byte("block"), 0o644))
	unwritable := filepath.Join(blocked, "subdir", "cache.json")

	resetProviderState()
	defer resetProviderState()

	// Prime both syncers with mock clients so Providers reuses the memoized
	// outcome instead of reaching the network.
	catwalkSyncer.Init(&mockCatwalkClient{
		providers: []catwalk.Provider{{Name: "Provider1", ID: "p1"}},
	}, unwritable, true)
	hyperSyncer.Init(&mockHyperClient{
		provider: catwalk.Provider{
			Name:   "Hyper",
			ID:     "hyper",
			Models: []catwalk.Model{{ID: "hyper-1", Name: "Hyper Model"}},
		},
	}, unwritable, true)

	catwalkProviders, catwalkErr := catwalkSyncer.Get(t.Context())
	require.Error(t, catwalkErr, "cache write should fail")
	require.NotEmpty(t, catwalkProviders, "syncer still returns a usable catalog")

	hyperProvider, hyperErr := hyperSyncer.Get(t.Context())
	require.Error(t, hyperErr, "cache write should fail")
	require.Equal(t, "Hyper", hyperProvider.Name)

	providers, err := Providers(&Config{Options: &Options{}})

	// The failure is reported, but as a warning alongside a usable catalog.
	require.Error(t, err)
	require.Len(t, providers, 2)
	require.Equal(t, catwalk.InferenceProvider("hyper"), providers[0].ID, "Hyper stays at the front")
	require.Equal(t, catwalk.InferenceProvider("p1"), providers[1].ID)
}

// TestProviders_FallsBackToEmbeddedHyper checks that Hyper is still in the
// catalog when it could not be fetched at all, using the copy bundled with
// this release.
func TestProviders_FallsBackToEmbeddedHyper(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	resetProviderState()
	defer resetProviderState()

	catwalkSyncer.Init(&mockCatwalkClient{
		providers: []catwalk.Provider{{Name: "Provider1", ID: "p1"}},
	}, filepath.Join(tmpDir, "providers.json"), true)
	hyperSyncer.Init(&mockHyperClient{
		err: errors.New("network error"),
	}, filepath.Join(tmpDir, "hyper.json"), true)

	_, _ = catwalkSyncer.Get(t.Context())
	_, _ = hyperSyncer.Get(t.Context())

	providers, err := Providers(&Config{Options: &Options{}})
	require.NoError(t, err)
	require.Len(t, providers, 2)
	require.Equal(t, catwalk.InferenceProvider("hyper"), providers[0].ID)
	require.NotEmpty(t, providers[0].Models, "the embedded Hyper provider carries models")
}

// TestProviders_HonorsDisableDefaultProviders makes sure the embedded Hyper
// fallback does not smuggle a default provider back in.
func TestProviders_HonorsDisableDefaultProviders(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	resetProviderState()
	defer resetProviderState()

	providers, err := Providers(&Config{
		Options: &Options{DisableDefaultProviders: true},
	})
	require.NoError(t, err)
	require.Empty(t, providers)
}

// TestCacheStore_ReplacesFileInsteadOfRewritingIt guards the property that
// several Crush instances depend on: the provider cache is swapped into place
// as a finished file, never truncated and refilled underneath a reader that is
// already reading it. A reader that loses that race cannot parse the catalog
// and silently falls back to the bundled copy.
func TestCacheStore_ReplacesFileInsteadOfRewritingIt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "providers.json")
	c := newCache[[]catwalk.Provider](path)

	require.NoError(t, c.Store([]catwalk.Provider{{ID: "first", Name: "First"}}))
	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, c.Store([]catwalk.Provider{{ID: "second", Name: "Second"}}))
	after, err := os.Stat(path)
	require.NoError(t, err)

	// os.Stat on Windows resolves file identity lazily by reopening the path,
	// so both stats describe whichever file the path points at by the time
	// they are compared and SameFile cannot observe the replacement. The
	// write path is shared, so asserting this on the other platforms covers
	// it. The checks below still run everywhere.
	if runtime.GOOS != "windows" {
		require.False(t, os.SameFile(before, after),
			"the cache should be replaced by a rename, not rewritten in place")
	}

	// The new contents are complete and no temporary files are left behind.
	got, _, err := c.Get()
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, catwalk.InferenceProvider("second"), got[0].ID)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "only the cache file should remain")
	require.Equal(t, "providers.json", entries[0].Name())
}
