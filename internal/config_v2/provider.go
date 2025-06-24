package configv2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/fur/pkg/client"
	"github.com/charmbracelet/fur/pkg/provider"
)

var fur = client.New()

var (
	providerOnc  sync.Once // Ensures the initialization happens only once
	providerList []provider.Provider
)

func providersPath() string {
	return filepath.Join(baseDataPath(), "providers.json")
}

func saveProviders(providers []provider.Provider) error {
	path := providersPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(providers, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func loadProviders() ([]provider.Provider, error) {
	path := providersPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var providers []provider.Provider
	err = json.Unmarshal(data, &providers)
	return providers, err
}

func Providers() []provider.Provider {
	providerOnc.Do(func() {
		// Try to get providers from upstream API
		if providers, err := fur.GetProviders(); err == nil {
			providerList = providers
			// Save providers locally for future fallback
			_ = saveProviders(providers)
		} else {
			// If upstream fails, try to load from local cache
			if localProviders, localErr := loadProviders(); localErr == nil {
				providerList = localProviders
			} else {
				// If both fail, return empty list
				providerList = []provider.Provider{}
			}
		}
	})
	return providerList
}
