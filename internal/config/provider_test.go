package config

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func resetProviderState() {
	providerOnce = sync.Once{}
	providerList = nil
}

func TestProviders_ReturnsEmbedded(t *testing.T) {
	resetProviderState()
	defer resetProviderState()

	cfg := &Config{
		Options: &Options{},
	}

	providers, err := Providers(cfg)
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.Greater(t, len(providers), 5, "Expected embedded providers")
}

func TestProviders_CalledMultipleTimesReturnsSame(t *testing.T) {
	resetProviderState()
	defer resetProviderState()

	cfg := &Config{
		Options: &Options{},
	}

	providers1, err := Providers(cfg)
	require.NoError(t, err)

	providers2, err := Providers(cfg)
	require.NoError(t, err)

	require.Equal(t, len(providers1), len(providers2))
}
