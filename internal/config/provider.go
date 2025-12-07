package config

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/catwalk/pkg/embedded"
	"github.com/charmbracelet/crush/internal/home"
)

const copilotModelsURL = "https://models.dev/api.json"

type modelsDevResponse map[string]modelsDevProvider

type modelsDevProvider struct {
	ID     string                    `json:"id"`
	Name   string                    `json:"name"`
	API    string                    `json:"api"`
	Doc    string                    `json:"doc"`
	Models map[string]modelsDevModel `json:"models"`
}

type modelsDevModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Attachment  bool   `json:"attachment"`
	Reasoning   bool   `json:"reasoning"`
	ToolCall    bool   `json:"tool_call"`
	Temperature bool   `json:"temperature"`
	Cost        struct {
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
	} `json:"cost"`
	Limit struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
	Status string `json:"status"`
}

var (
	providerOnce sync.Once
	providerList []catwalk.Provider
	providerErr  error
)

type ProviderClient interface {
	GetProviders() ([]catwalk.Provider, error)
}

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
	slog.Info("Saving provider data to disk", "path", path)
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

func UpdateProviders(pathOrUrl string) error {
	var providers []catwalk.Provider
	pathOrUrl = cmp.Or(pathOrUrl, os.Getenv("CATWALK_URL"), defaultCatwalkURL)

	switch {
	case pathOrUrl == "embedded":
		providers = embedded.GetAll()
	case strings.HasPrefix(pathOrUrl, "http://") || strings.HasPrefix(pathOrUrl, "https://"):
		var err error
		providers, err = catwalk.NewWithURL(pathOrUrl).GetProviders()
		if err != nil {
			return fmt.Errorf("failed to fetch providers from Catwalk: %w", err)
		}
	default:
		content, err := os.ReadFile(pathOrUrl)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if err := json.Unmarshal(content, &providers); err != nil {
			return fmt.Errorf("failed to unmarshal provider data: %w", err)
		}
		if len(providers) == 0 {
			return fmt.Errorf("no providers found in the provided source")
		}
	}

	cachePath := providerCacheFileData()
	if err := saveProvidersInCache(cachePath, providers); err != nil {
		return fmt.Errorf("failed to save providers to cache: %w", err)
	}

	slog.Info("Providers updated successfully", "count", len(providers), "from", pathOrUrl, "to", cachePath)
	return nil
}

func Providers(cfg *Config) ([]catwalk.Provider, error) {
	providerOnce.Do(func() {
		catwalkURL := cmp.Or(os.Getenv("CATWALK_URL"), defaultCatwalkURL)
		client := catwalk.NewWithURL(catwalkURL)
		path := providerCacheFileData()

		autoUpdateDisabled := cfg.Options.DisableProviderAutoUpdate
		providerList, providerErr = loadProviders(autoUpdateDisabled, client, path)
	})
	return providerList, providerErr
}

func loadProviders(autoUpdateDisabled bool, client ProviderClient, path string) ([]catwalk.Provider, error) {
	catwalkGetAndSave := func() ([]catwalk.Provider, error) {
		providers, err := client.GetProviders()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch providers from catwalk: %w", err)
		}
		if len(providers) == 0 {
			return nil, fmt.Errorf("empty providers list from catwalk")
		}
		if err := saveProvidersInCache(path, providers); err != nil {
			return nil, err
		}
		return providers, nil
	}

	switch {
	case autoUpdateDisabled:
		slog.Warn("Providers auto-update is disabled")

		if _, err := os.Stat(path); err == nil {
			slog.Warn("Using locally cached providers")
			return loadProvidersFromCache(path)
		}

		slog.Warn("Saving embedded providers to cache")
		providers := embedded.GetAll()
		if err := saveProvidersInCache(path, providers); err != nil {
			return nil, err
		}
		return providers, nil

	default:
		slog.Info("Fetching providers from Catwalk.", "path", path)

		providers, err := catwalkGetAndSave()
		if err != nil {
			catwalkUrl := fmt.Sprintf("%s/v2/providers", cmp.Or(os.Getenv("CATWALK_URL"), defaultCatwalkURL))
			return nil, fmt.Errorf("Crush was unable to fetch an updated list of providers from %s. Consider setting CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1 to use the embedded providers bundled at the time of this Crush release. You can also update providers manually. For more info see crush update-providers --help. %w", catwalkUrl, err) //nolint:staticcheck
		}
		return providers, nil
	}
}

func FetchCopilotModels() ([]catwalk.Model, error) {
	req, err := http.NewRequestWithContext(context.TODO(), "GET", copilotModelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev returned status %d", resp.StatusCode)
	}

	var data modelsDevResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode models: %w", err)
	}

	provider, ok := data["github-copilot"]
	if !ok {
		return nil, fmt.Errorf("github-copilot not found in models.dev")
	}

	var models []catwalk.Model
	for _, m := range provider.Models {
		if m.Status == "deprecated" {
			continue
		}

		models = append(models, catwalk.Model{
			ID:               m.ID,
			Name:             m.Name,
			CostPer1MIn:      m.Cost.Input,
			CostPer1MOut:     m.Cost.Output,
			ContextWindow:    int64(m.Limit.Context),
			DefaultMaxTokens: int64(m.Limit.Output),
			CanReason:        m.Reasoning,
			SupportsImages:   m.Attachment,
		})
	}

	return models, nil
}

func UpdateCopilotModels() error {
	cfg := Get()
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	providerCfg, ok := cfg.Providers.Get("github-copilot")
	if !ok || providerCfg.OAuthToken == nil {
		return fmt.Errorf("github-copilot not authenticated")
	}

	models, err := FetchCopilotModels()
	if err != nil {
		return err
	}

	providerCfg.Models = models
	cfg.Providers.Set("github-copilot", providerCfg)

	slog.Info("Updated GitHub Copilot models", "count", len(models))
	return nil
}
