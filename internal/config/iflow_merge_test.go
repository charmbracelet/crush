package config

import (
	"sync"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestProviders_IFlowMerge(t *testing.T) {
	// Reset global state
	originalCatwalkSyncer := catwalkSyncer
	originalHyperSyncer := hyperSyncer
	originalProviderOnce := providerOnce
	originalProviderList := providerList
	originalProviderErr := providerErr

	defer func() {
		catwalkSyncer = originalCatwalkSyncer
		hyperSyncer = originalHyperSyncer
		providerOnce = originalProviderOnce
		providerList = originalProviderList
		providerErr = originalProviderErr
	}()

	t.Run("should merge built-in models if iflow already exists in catwalk", func(t *testing.T) {
		providerOnce = sync.Once{}
		catwalkSyncer = &catwalkSync{}
		hyperSyncer = &hyperSync{}

		tmpDir := t.TempDir()
		path := tmpDir + "/providers.json"

		// Mock catwalk client returns iflow provider with one existing model
		existingModel := catwalk.Model{ID: "existing-model", Name: "Existing Model"}
		mockClient := &mockCatwalkClient{
			providers: []catwalk.Provider{
				{
					ID:     InferenceProviderIFlow,
					Name:   "iFlow",
					Models: []catwalk.Model{existingModel},
				},
			},
		}
		catwalkSyncer.Init(mockClient, path, true)

		// Mock hyper syncer (disabled for simplicity)
		t.Setenv("CRUSH_HYPER_DISABLED", "1")

		cfg := &Config{
			Options: &Options{
				DisableProviderAutoUpdate: false,
			},
		}

		providers, err := Providers(cfg)
		require.NoError(t, err)

		var iflow catwalk.Provider
		found := false
		for _, p := range providers {
			if p.ID == InferenceProviderIFlow {
				iflow = p
				found = true
				break
			}
		}

		require.True(t, found, "iFlow provider should be found")

		// Check that models were merged
		// Should have the existing model + the 4 built-in models
		require.Len(t, iflow.Models, 5)

		modelIDs := make(map[string]bool)
		for _, m := range iflow.Models {
			modelIDs[m.ID] = true
		}

		require.True(t, modelIDs["existing-model"])
		require.True(t, modelIDs["minimax-m2.1"])
		require.True(t, modelIDs["deepseek-v3.2"])
		require.True(t, modelIDs["glm-4.7"])
		require.True(t, modelIDs["kimi-k2-thinking"])
	})

	t.Run("should add iflow provider if not exists in catwalk", func(t *testing.T) {
		providerOnce = sync.Once{}
		catwalkSyncer = &catwalkSync{}
		hyperSyncer = &hyperSync{}

		tmpDir := t.TempDir()
		path := tmpDir + "/providers.json"

		// Mock catwalk client returns other providers
		mockClient := &mockCatwalkClient{
			providers: []catwalk.Provider{
				{
					ID:   "other-provider",
					Name: "Other Provider",
				},
			},
		}
		catwalkSyncer.Init(mockClient, path, true)

		cfg := &Config{
			Options: &Options{
				DisableProviderAutoUpdate: false,
			},
		}

		providers, err := Providers(cfg)
		require.NoError(t, err)

		var iflow catwalk.Provider
		found := false
		for _, p := range providers {
			if p.ID == InferenceProviderIFlow {
				iflow = p
				found = true
				break
			}
		}

		// Check that iflow provider was added
		require.True(t, found, "iFlow provider should be found")

		// Should have the 4 built-in models
		require.Len(t, iflow.Models, 4)

		modelIDs := make(map[string]bool)
		for _, m := range iflow.Models {
			modelIDs[m.ID] = true
		}

		require.True(t, modelIDs["minimax-m2.1"])
		require.True(t, modelIDs["deepseek-v3.2"])
		require.True(t, modelIDs["glm-4.7"])
		require.True(t, modelIDs["kimi-k2-thinking"])
	})
}
