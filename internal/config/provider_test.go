package config

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProviderClient struct {
	shouldFail bool
}

func (m *mockProviderClient) GetProviders() ([]provider.Provider, error) {
	if m.shouldFail {
		return nil, errors.New("failed to load providers")
	}
	return []provider.Provider{
		{
			Name: "Mock",
		},
	}, nil
}

func TestProvider_loadProvidersNoIssues(t *testing.T) {
	client := &mockProviderClient{shouldFail: false}
	tmpPath := t.TempDir() + "/providers.json"
	providers, err := loadProviders(tmpPath, client)
	assert.NoError(t, err)
	assert.NotNil(t, providers)
	assert.Len(t, providers, 1)

	// check if file got saved
	fileInfo, err := os.Stat(tmpPath)
	assert.NoError(t, err)
	assert.False(t, fileInfo.IsDir(), "Expected a file, not a directory")
}

func TestProvider_loadProvidersWithIssues(t *testing.T) {
	client := &mockProviderClient{shouldFail: true}
	tmpPath := t.TempDir() + "/providers.json"
	// store providers to a temporary file
	oldProviders := []provider.Provider{
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
	providers, err := loadProviders(tmpPath, client)
	assert.NoError(t, err)
	assert.NotNil(t, providers)
	require.Len(t, providers, 1)
	assert.Equal(t, "OldProvider", providers[0].Name, "Expected to keep old provider when loading fails")
}

func TestProvider_loadProvidersWithIssuesAndNoCache(t *testing.T) {
	client := &mockProviderClient{shouldFail: true}
	tmpPath := t.TempDir() + "/providers.json"
	providers, err := loadProviders(tmpPath, client)
	assert.Error(t, err)
	assert.Nil(t, providers, "Expected nil providers when loading fails and no cache exists")
}
