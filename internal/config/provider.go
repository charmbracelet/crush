package config

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/home"
)

type ProviderClient interface {
	GetProviders() ([]catwalk.Provider, error)
}

var (
	providerOnce sync.Once
	providerList []catwalk.Provider
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

	return filepath.Join(home.Dir(), ".local", "share", appName, "providers.json")
}

func saveProvidersInCache(path string, providers []catwalk.Provider) error {
	slog.Info("Saving cached provider data", "path", path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for provider cache: %w", err)
	}

	data, err := json.MarshalIndent(providers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write provider data to cache: %w", err)
	}
	return nil
}

func loadProvidersFromCache(path string) ([]catwalk.Provider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider cache file: %w", err)
	}

	var providers []catwalk.Provider
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provider data from cache: %w", err)
	}
	return providers, nil
}

func Providers() ([]catwalk.Provider, error) {
	catwalkURL := cmp.Or(os.Getenv("CATWALK_URL"), defaultCatwalkURL)
	client := catwalk.NewWithURL(catwalkURL)
	path := providerCacheFileData()
	return loadProvidersOnce(client, path)
}

func loadProvidersOnce(client ProviderClient, path string) ([]catwalk.Provider, error) {
	var err error
	providerOnce.Do(func() {
		providerList, err = loadProviders(client, path)
	})
	if err != nil {
		return nil, err
	}
	return providerList, nil
}

func loadProviders(client ProviderClient, path string) (providerList []catwalk.Provider, err error) {
	// if cache is not stale, load from it
	stale, exists := isCacheStale(path)
	if !stale {
		slog.Info("Using cached provider data", "path", path)
		providerList, err = loadProvidersFromCache(path)
		if len(providerList) > 0 && err == nil {
			go func() {
				slog.Info("Updating provider cache in background", "path", path)
				updated, uerr := client.GetProviders()
				if len(updated) > 0 && uerr == nil {
					_ = saveProvidersInCache(path, updated)
				}
			}()
			return
		}
	}

	slog.Info("Getting live provider data", "path", path)
	providerList, err = client.GetProviders()
	if len(providerList) > 0 && err == nil {
		err = saveProvidersInCache(path, providerList)
		return
	}
	if !exists {
		err = fmt.Errorf("failed to load providers")
		return
	}
	slog.Info("Loading provider data from cache", "path", path)
	providerList, err = loadProvidersFromCache(path)
	return
}

func isCacheStale(path string) (stale, exists bool) {
	info, err := os.Stat(path)
	if err != nil {
		return true, false
	}
	return time.Since(info.ModTime()) > 24*time.Hour, true
}
