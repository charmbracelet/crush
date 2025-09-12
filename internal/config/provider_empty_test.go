package config

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

type emptyProviderClient struct{}

func (m *emptyProviderClient) GetProviders() ([]catwalk.Provider, error) {
	return []catwalk.Provider{}, nil
}

func TestProvider_loadProvidersEmptyResult(t *testing.T) {
	client := &emptyProviderClient{}
	tmpPath := t.TempDir() + "/providers.json"

	providers, err := loadProviders(false, client, tmpPath)
	require.Contains(t, err.Error(), "Crush was unable to fetch an updated list of providers")
	require.Empty(t, providers)
	require.Len(t, providers, 0)

	// Check that no cache file was created for empty results
	require.NoFileExists(t, tmpPath, "Cache file should not exist for empty results")
}

func TestProvider_loadProvidersEmptyCache(t *testing.T) {
	client := &mockProviderClient{shouldFail: false}
	tmpPath := t.TempDir() + "/providers.json"

	// Create an empty cache file
	emptyProviders := []catwalk.Provider{}
	data, err := json.Marshal(emptyProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpPath, data, 0o644))

	// Should refresh and get real providers instead of using empty cache
	providers, err := loadProviders(false, client, tmpPath)
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.Len(t, providers, 1)
	require.Equal(t, "Mock", providers[0].Name)
}
