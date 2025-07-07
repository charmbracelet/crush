package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/charmbracelet/crush/internal/fur/client"
	"github.com/charmbracelet/crush/internal/fur/provider"
)

type ProviderClient interface {
	GetProviders() ([]provider.Provider, error)
}

var (
	providerOnce sync.Once
	providerList []provider.Provider
)

// file to cache provider data
func providerCacheFileData() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		return filepath.Join(xdgDataHome, appName, "providers.json")
	}

	// return the path to the main data directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.local/share/crush/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName, "providers.json")
	}

	return filepath.Join(os.Getenv("HOME"), ".local", "share", appName, "providers.json")
}

func saveProvidersInCache(path string, providers []provider.Provider) error {
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

func loadProvidersFromCache(path string) ([]provider.Provider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var providers []provider.Provider
	err = json.Unmarshal(data, &providers)
	return providers, err
}

func loadProviders(path string, client ProviderClient) ([]provider.Provider, error) {
	providers, err := client.GetProviders()
	if err != nil {
		fallbackToCache, err := loadProvidersFromCache(path)
		if err != nil {
			return nil, err
		}
		providers = fallbackToCache
	} else {
		if err := saveProvidersInCache(path, providerList); err != nil {
			return nil, err
		}
	}
	return providers, nil
}

func Providers() ([]provider.Provider, error) {
	return LoadProviders(client.New())
}

func LoadProviders(client ProviderClient) ([]provider.Provider, error) {
	var err error
	providerOnce.Do(func() {
		providerList, err = loadProviders(providerCacheFileData(), client)
	})
	if err != nil {
		return nil, err
	}
	return providerList, nil
}
