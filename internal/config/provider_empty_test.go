package config

import (
	"context"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

type emptyProviderClient struct{}

func (m *emptyProviderClient) GetProviders(context.Context, string) ([]catwalk.Provider, error) {
	return []catwalk.Provider{}, nil
}

// TestProvider_loadProvidersEmptyResult tests that loadProviders returns an
// error when the client returns an empty list. This ensures we don't cache
// empty provider lists.
func TestProvider_loadProvidersEmptyResult(t *testing.T) {
	client := &emptyProviderClient{}
	tmpPath := t.TempDir() + "/providers.json"

	providers, err := loadProviders(client, "", tmpPath)
	require.Contains(t, err.Error(), "empty providers list from catwalk")
	require.Empty(t, providers)
	require.Len(t, providers, 0)

	// Check that no cache file was created for empty results
	require.NoFileExists(t, tmpPath, "Cache file should not exist for empty results")
}
