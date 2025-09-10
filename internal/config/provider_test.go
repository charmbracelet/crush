package config

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

type mockProviderClient struct {
	shouldFail bool
}

func (m *mockProviderClient) GetProviders() ([]catwalk.Provider, error) {
	if m.shouldFail {
		return nil, errors.New("failed to load providers")
	}
	return []catwalk.Provider{
		{
			Name: "Mock",
		},
	}, nil
}

func TestProvider_loadProvidersNoIssues(t *testing.T) {
	client := &mockProviderClient{shouldFail: false}
	tmpPath := t.TempDir() + "/providers.json"
	providers, err := loadProviders(false, client, tmpPath)
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.Len(t, providers, 1)

	// check if file got saved
	fileInfo, err := os.Stat(tmpPath)
	require.NoError(t, err)
	require.False(t, fileInfo.IsDir(), "Expected a file, not a directory")
}

func TestProvider_loadProvidersWithIssues(t *testing.T) {
	client := &mockProviderClient{shouldFail: true}
	tmpPath := t.TempDir() + "/providers.json"
	// store providers to a temporary file
	oldProviders := []catwalk.Provider{
		{
			Name: "OldProvider",
		},
	}
	data, err := json.Marshal(oldProviders)
	if err != nil {
		t.Fatalf("Failed to marshal old providers: %v", err)
	}

	err = os.WriteFile(tmpPath, data, 0o644)
	if err != nil {
		t.Fatalf("Failed to write old providers to file: %v", err)
	}
	providers, err := loadProviders(false, client, tmpPath)
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.Len(t, providers, 1)
	require.Equal(t, "OldProvider", providers[0].Name, "Expected to keep old provider when loading fails")
}

func TestProvider_loadProvidersWithIssuesAndNoCache(t *testing.T) {
	client := &mockProviderClient{shouldFail: true}
	tmpPath := t.TempDir() + "/providers.json"
	providers, err := loadProviders(false, client, tmpPath)
	require.Error(t, err)
	require.Nil(t, providers, "Expected nil providers when loading fails and no cache exists")
}
