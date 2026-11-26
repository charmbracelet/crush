package config

import (
	"context"
	"os"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

type emptyProviderClient struct{}

func (m *emptyProviderClient) GetProviders(context.Context, string) ([]catwalk.Provider, []byte, error) {
	return []catwalk.Provider{}, []byte("[]"), nil
}

// TestCatwalkSync_GetEmptyResultFromClient tests that when the client returns
// an empty list, we fall back to cached providers and return an error.
func TestCatwalkSync_GetEmptyResultFromClient(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/providers.json"

	syncer := newCatwalkSync()
	client := &emptyProviderClient{}

	syncer.Init(client.GetProviders, path, true)

	providers, err := syncer.Get(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty Catwalk providers")
	require.NotEmpty(t, providers) // Should have embedded providers as fallback.

	// Check that no cache file was created for empty results.
	_, statErr := os.Stat(path)
	require.True(t, os.IsNotExist(statErr), "Cache file should not exist for empty results")
}
