package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

type mockProviderClient struct {
	shouldFail      bool
	shouldReturnErr error
}

func (m *mockProviderClient) GetProviders(context.Context, string) ([]catwalk.Provider, error) {
	if m.shouldReturnErr != nil {
		return nil, m.shouldReturnErr
	}
	if m.shouldFail {
		return nil, errors.New("failed to load providers")
	}
	return []catwalk.Provider{
		{
			Name: "Mock",
		},
	}, nil
}

func resetProviderState() {
	providerOnce = sync.Once{}
	providerList = nil
	providerErr = nil
}

func TestProvider_loadProvidersNoIssues(t *testing.T) {
	client := &mockProviderClient{shouldFail: false}
	tmpPath := t.TempDir() + "/providers.json"
	providers, err := loadProviders(client, "", tmpPath)
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.Len(t, providers, 1)

	// check if file got saved
	fileInfo, err := os.Stat(tmpPath)
	require.NoError(t, err)
	require.False(t, fileInfo.IsDir(), "Expected a file, not a directory")
}

func TestProvider_DisableAutoUpdate(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

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

func TestProvider_WithValidCache(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	resetProviderState()
	defer resetProviderState()

	cachePath := tmpDir + "/crush/providers.json"
	require.NoError(t, os.MkdirAll(tmpDir+"/crush", 0o755))
	cachedProviders := []catwalk.Provider{
		{Name: "Cached"},
	}
	data, err := json.Marshal(cachedProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0o644))

	mockClient := &mockProviderClient{shouldFail: false}

	providers, err := loadProviders(mockClient, "", cachePath)
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.Len(t, providers, 1)
	require.Equal(t, "Mock", providers[0].Name, "Expected fresh provider from fetch")
}

func TestProvider_NotModifiedUsesCached(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	resetProviderState()
	defer resetProviderState()

	cachePath := tmpDir + "/crush/providers.json"
	require.NoError(t, os.MkdirAll(tmpDir+"/crush", 0o755))
	cachedProviders := []catwalk.Provider{
		{Name: "Cached"},
	}
	data, err := json.Marshal(cachedProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0o644))

	mockClient := &mockProviderClient{shouldReturnErr: catwalk.ErrNotModified}
	providers, err := loadProviders(mockClient, "", cachePath)
	require.ErrorIs(t, err, catwalk.ErrNotModified)
	require.Nil(t, providers)
}

func TestProvider_EmptyCacheDefaultsToEmbedded(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	resetProviderState()
	defer resetProviderState()

	cachePath := tmpDir + "/crush/providers.json"
	require.NoError(t, os.MkdirAll(tmpDir+"/crush", 0o755))
	emptyProviders := []catwalk.Provider{}
	data, err := json.Marshal(emptyProviders)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0o644))

	cached, _, err := loadProvidersFromCache(cachePath)
	require.NoError(t, err)
	require.Empty(t, cached, "Expected empty cache")
}

func TestProvider_loadProvidersWithIssuesAndNoCache(t *testing.T) {
	client := &mockProviderClient{shouldFail: true}
	tmpPath := t.TempDir() + "/providers.json"
	providers, err := loadProviders(client, "", tmpPath)
	require.Error(t, err)
	require.Nil(t, providers, "Expected nil providers when loading fails and no cache exists")
}
