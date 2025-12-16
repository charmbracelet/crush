package config

import (
	"context"
	"os"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

type emptyProviderClient struct{}

func (m *emptyProviderClient) GetProviders(context.Context, string) ([]catwalk.Provider, error) {
	return []catwalk.Provider{}, nil
}

// TestCatwalkSync_GetEmptyResultFromClient tests that when the client returns
// an empty list, we fall back to cached providers and return an error.
func TestCatwalkSync_GetEmptyResultFromClient(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir + "/providers.json"

	syncer := &catwalkSync{}
	client := &emptyProviderClient{}

	syncer.Init(client, path, true)

	providers, err := syncer.Get(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty providers list from catwalk")
	require.NotEmpty(t, providers) // Should have embedded providers as fallback.

	// Check that no cache file was created for empty results.
	_, statErr := os.Stat(path)
	require.True(t, os.IsNotExist(statErr), "Cache file should not exist for empty results")
}
