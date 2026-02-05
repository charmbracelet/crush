package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

type mockHyperClient struct {
	provider  catwalk.Provider
	err       error
	callCount int
}

func (m *mockHyperClient) Get(ctx context.Context, etag string) (catwalk.Provider, error) {
	m.callCount++
	return m.provider, m.err
}

func TestHyperSync_Init(t *testing.T) {
	t.Parallel()

	syncer := &hyperSync{}
	client := &mockHyperClient{}
	path := "/tmp/hyper.json"

	syncer.Init(client, path, true)

	require.True(t, syncer.init.Load())
	require.Equal(t, client, syncer.client)
	require.Equal(t, path, syncer.cache.path)
}

func TestHyperSync_GetPanicIfNotInit(t *testing.T) {
	t.Parallel()

	syncer := &hyperSync{}
	require.Panics(t, func() {
		_, _ = syncer.Get(t.Context())
	})
}

func TestHyperSync_GetFreshProvider(t *testing.T) {
	t.Parallel()

	syncer := &hyperSync{}
	client := &mockHyperClient{
		provider: catwalk.Provider{
			Name: "Hyper",
			ID:   "hyper",
			Models: []catwalk.Model{
				{ID: "model-1", Name: "Model 1"},
			},
		},
	}
	path := t.TempDir() + "/hyper.json"

	syncer.Init(client, path, true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Hyper", provider.Name)
	require.Equal(t, 1, client.callCount)

	// Verify cache was written.
	fileInfo, err := os.Stat(path)
	require.NoError(t, err)
	require.False(t, fileInfo.IsDir())
}

func TestHyperSync_GetNotModifiedUsesCached(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/hyper.json"

	// Create cache file.
	cachedProvider := catwalk.Provider{
		Name: "Cached Hyper",
		ID:   "hyper",
	}
	data, err := json.Marshal(cachedProvider)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	syncer := &hyperSync{}
	client := &mockHyperClient{
		err: catwalk.ErrNotModified,
	}

	syncer.Init(client, path, true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Cached Hyper", provider.Name)
	require.Equal(t, 1, client.callCount)
}

func TestHyperSync_GetClientError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/hyper.json"

	syncer := &hyperSync{}
	client := &mockHyperClient{
		err: errors.New("network error"),
	}

	syncer.Init(client, path, true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err) // Should fall back to embedded.
	require.Equal(t, "Charm Hyper", provider.Name)
	require.Equal(t, catwalk.InferenceProvider("hyper"), provider.ID)
}

func TestHyperSync_GetEmptyCache(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/hyper.json"

	syncer := &hyperSync{}
	client := &mockHyperClient{
		provider: catwalk.Provider{
			Name: "Fresh Hyper",
			ID:   "hyper",
			Models: []catwalk.Model{
				{ID: "model-1", Name: "Model 1"},
			},
		},
	}

	syncer.Init(client, path, true)

	provider, err := syncer.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Fresh Hyper", provider.Name)
}

func TestHyperSync_GetCalledMultipleTimesUsesOnce(t *testing.T) {
	t.Parallel()

	syncer := &hyperSync{}
	client := &mockHyperClient{
		provider: catwalk.Provider{
			Name: "Hyper",
			ID:   "hyper",
			Models: []catwalk.Model{
				{ID: "model-1", Name: "Model 1"},
			},
		},
	}
	path := t.TempDir() + "/hyper.json"

	syncer.Init(client, path, true)

	// Call Get multiple times.
	provider1, err1 := syncer.Get(t.Context())
	require.NoError(t, err1)
	require.Equal(t, "Hyper", provider1.Name)

	provider2, err2 := syncer.Get(t.Context())
	require.NoError(t, err2)
	require.Equal(t, "Hyper", provider2.Name)

	// Client should only be called once due to sync.Once.
	require.Equal(t, 1, client.callCount)
}

func TestHyperSync_GetCacheStoreError(t *testing.T) {
	t.Parallel()

	// Create a file where we want a directory, causing mkdir to fail.
	tmpDir := t.TempDir()
	blockingFile := tmpDir + "/blocking"
	require.NoError(t, os.WriteFile(blockingFile, []byte("block"), 0o644))

	// Try to create cache in a subdirectory under the blocking file.
	path := blockingFile + "/subdir/hyper.json"

	syncer := &hyperSync{}
	client := &mockHyperClient{
		provider: catwalk.Provider{
			Name: "Hyper",
			ID:   "hyper",
			Models: []catwalk.Model{
				{ID: "model-1", Name: "Model 1"},
			},
		},
	}

	syncer.Init(client, path, true)

	provider, err := syncer.Get(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create directory for provider cache")
	require.Equal(t, "Hyper", provider.Name) // Provider is still returned.
}
