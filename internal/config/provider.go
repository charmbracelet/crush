package config

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/catwalk/pkg/embedded"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/x/etag"
)

type syncer[T any] interface {
	Get(context.Context) (T, error)
}

var (
	providerOnce sync.Once
	providerList []catwalk.Provider
	providerErr  error
)

var errMissingLiveProviderCredentials = errors.New("missing live provider credentials")

// IsMissingLiveProviderCredentials reports whether err means a live provider
// cannot be updated because no credentials are configured.
func IsMissingLiveProviderCredentials(err error) bool {
	return errors.Is(err, errMissingLiveProviderCredentials)
}

// file to cache provider data
func cachePathFor(name string) string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		return filepath.Join(xdgDataHome, appName, name+".json")
	}

	// return the path to the main data directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.local/share/crush/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName, name+".json")
	}

	return filepath.Join(home.Dir(), ".local", "share", appName, name+".json")
}

// UpdateProviders updates the Catwalk providers list from a specified source.
func UpdateProviders(pathOrURL string) error {
	var providers []catwalk.Provider
	pathOrURL = cmp.Or(pathOrURL, os.Getenv("CATWALK_URL"), defaultCatwalkURL)

	switch {
	case pathOrURL == "embedded":
		providers = embedded.GetAll()
	case strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://"):
		var err error
		providers, err = catwalk.NewWithURL(pathOrURL).GetProviders(context.Background(), "")
		if err != nil {
			return fmt.Errorf("failed to fetch providers from Catwalk: %w", err)
		}
	default:
		content, err := os.ReadFile(pathOrURL)
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

	if err := newCache[[]catwalk.Provider](cachePathFor("providers")).Store(providers); err != nil {
		return fmt.Errorf("failed to save providers to cache: %w", err)
	}

	slog.Info("Providers updated successfully", "count", len(providers), "from", pathOrURL, "to", cachePathFor("providers"))
	return nil
}

// UpdateHyper updates the Hyper provider information from a specified URL.
func UpdateHyper(pathOrURL string) error {
	var provider catwalk.Provider
	pathOrURL = cmp.Or(pathOrURL, hyper.BaseURL())

	switch {
	case pathOrURL == "embedded":
		provider = hyper.Embedded()
	case strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://"):
		client := realHyperClient{baseURL: pathOrURL}
		var err error
		provider, err = client.Get(context.Background(), "")
		if err != nil {
			return fmt.Errorf("failed to fetch provider from Hyper: %w", err)
		}
	default:
		content, err := os.ReadFile(pathOrURL)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if err := json.Unmarshal(content, &provider); err != nil {
			return fmt.Errorf("failed to unmarshal provider data: %w", err)
		}
	}

	if err := newCache[catwalk.Provider](cachePathFor("hyper")).Store(provider); err != nil {
		return fmt.Errorf("failed to save Hyper provider to cache: %w", err)
	}

	slog.Info("Hyper provider updated successfully", "from", pathOrURL, "to", cachePathFor("hyper"))
	return nil
}

// UpdateVenice updates the cached Venice provider from a live source, embedded
// seed, or local provider file.
func UpdateVenice(pathOrURL string, cfg *Config) error {
	return updateLiveProvider("Venice", "venice", catwalk.InferenceProviderVenice, pathOrURL, cfg, newVeniceLiveProviderClient)
}

// UpdateCopilot updates the cached Copilot provider from a live source,
// embedded seed, or local provider file.
func UpdateCopilot(pathOrURL string, cfg *Config) error {
	return updateLiveProvider("Copilot", "copilot", catwalk.InferenceProviderCopilot, pathOrURL, cfg, newCopilotLiveProviderClient)
}

var (
	catwalkSyncer = &catwalkSync{}
	hyperSyncer   = &hyperSync{}
	veniceSyncer  = &liveProviderSync{}
	copilotSyncer = &liveProviderSync{}
)

// Providers returns the list of providers, taking into account cached results
// and whether or not auto update is enabled.
//
// It will:
// 1. if auto update is disabled, it'll return the embedded providers at the
// time of release.
// 2. load the cached providers
// 3. try to get the fresh list of providers, and return either this new list,
// the cached list, or the embedded list if all others fail.
func Providers(cfg *Config) ([]catwalk.Provider, error) {
	providerOnce.Do(func() {
		var wg sync.WaitGroup
		var errs []error
		providers := csync.NewSlice[catwalk.Provider]()
		options := &Options{}
		if cfg != nil && cfg.Options != nil {
			options = cfg.Options
		}
		autoupdate := !options.DisableProviderAutoUpdate
		customProvidersOnly := options.DisableDefaultProviders

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		var hyperProvider catwalk.Provider
		var hyperFound bool

		wg.Go(func() {
			if customProvidersOnly {
				return
			}
			catwalkURL := cmp.Or(os.Getenv("CATWALK_URL"), defaultCatwalkURL)
			client := catwalk.NewWithURL(catwalkURL)
			path := cachePathFor("providers")
			catwalkSyncer.Init(client, path, autoupdate)

			items, err := catwalkSyncer.Get(ctx)
			if err != nil {
				catwalkURL := fmt.Sprintf("%s/v2/providers", cmp.Or(os.Getenv("CATWALK_URL"), defaultCatwalkURL))
				errs = append(errs, fmt.Errorf("Crush was unable to fetch an updated list of providers from %s. Consider setting CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1 to use the embedded providers bundled at the time of this Crush release. You can also update providers manually. For more info see crush update-providers --help.\n\nCause: %w", catwalkURL, err)) //nolint:staticcheck
				return
			}
			providers.Append(items...)
		})

		wg.Go(func() {
			if customProvidersOnly {
				return
			}
			path := cachePathFor("hyper")
			hyperSyncer.Init(realHyperClient{baseURL: hyper.BaseURL()}, path, autoupdate)

			item, err := hyperSyncer.Get(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("Crush was unable to fetch updated information from Hyper: %w", err)) //nolint:staticcheck
				return
			}
			hyperProvider = item
			hyperFound = true
		})

		wg.Wait()

		items := slices.Collect(providers.Seq())
		if !customProvidersOnly {
			var liveErrs []error
			items, liveErrs = overlayLiveProviderModels(ctx, cfg, items, autoupdate)
			errs = append(errs, liveErrs...)
		}

		if hyperFound {
			providerList = append([]catwalk.Provider{hyperProvider}, items...)
		} else {
			providerList = items
		}
		providerErr = errors.Join(errs...)
	})
	return providerList, providerErr
}

func overlayLiveProviderModels(ctx context.Context, cfg *Config, providers []catwalk.Provider, autoupdate bool) ([]catwalk.Provider, []error) {
	if cfg == nil || len(providers) == 0 {
		return providers, nil
	}

	environment := env.New()
	resolver := NewShellVariableResolver(environment)
	errs := make([]error, 0, 2)

	syncProvider := func(providerID catwalk.InferenceProvider, cacheName string, syncer *liveProviderSync, newClient liveProviderClientFunc) {
		index := slices.IndexFunc(providers, func(provider catwalk.Provider) bool {
			return provider.ID == providerID
		})
		if index < 0 {
			return
		}

		seed := providers[index]
		client, credentialed, err := newClient(seed, cfg, resolver, "")
		if err != nil {
			slog.Warn("Skipping live provider sync", "provider", providerID, "error", err)
			return
		}

		syncer.Init(client, cachePathFor(cacheName), autoupdate, seed, credentialed)
		provider, err := syncer.Get(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("Crush was unable to cache updated models from %s: %w", seed.Name, err))
			return
		}
		providers[index] = provider
	}

	syncProvider(catwalk.InferenceProviderVenice, "venice", veniceSyncer, newVeniceLiveProviderClient)
	syncProvider(catwalk.InferenceProviderCopilot, "copilot", copilotSyncer, newCopilotLiveProviderClient)

	return providers, errs
}

type liveProviderClientFunc func(catwalk.Provider, *Config, VariableResolver, string) (liveProviderClient, bool, error)

func newVeniceLiveProviderClient(seed catwalk.Provider, cfg *Config, resolver VariableResolver, baseURLOverride string) (liveProviderClient, bool, error) {
	var providerConfig ProviderConfig
	configExists := cfg != nil && cfg.Providers != nil
	if configExists {
		providerConfig, configExists = cfg.Providers.Get(string(catwalk.InferenceProviderVenice))
	}
	if configExists && providerConfig.Disable {
		return realVeniceModelsClient{baseURL: seed.APIEndpoint}, false, nil
	}

	baseURL := cmp.Or(baseURLOverride, seed.APIEndpoint)
	if configExists && providerConfig.BaseURL != "" && baseURLOverride == "" {
		resolved, err := resolver.ResolveValue(providerConfig.BaseURL)
		if err != nil {
			return nil, false, fmt.Errorf("failed to resolve Venice base URL: %w", err)
		}
		baseURL = resolved
	}

	apiKey := ""
	if configExists && providerConfig.APIKey != "" {
		resolved, err := resolver.ResolveValue(providerConfig.APIKey)
		if err != nil {
			return nil, false, fmt.Errorf("failed to resolve Venice API key: %w", err)
		}
		apiKey = strings.TrimSpace(resolved)
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("VENICE_API_KEY"))
	}

	return realVeniceModelsClient{baseURL: baseURL, apiKey: apiKey}, apiKey != "", nil
}

func newCopilotLiveProviderClient(seed catwalk.Provider, cfg *Config, resolver VariableResolver, baseURLOverride string) (liveProviderClient, bool, error) {
	var providerConfig ProviderConfig
	configExists := cfg != nil && cfg.Providers != nil
	if configExists {
		providerConfig, configExists = cfg.Providers.Get(string(catwalk.InferenceProviderCopilot))
	}
	if configExists && providerConfig.Disable {
		return &realCopilotModelsClient{baseURL: seed.APIEndpoint}, false, nil
	}

	baseURL := cmp.Or(baseURLOverride, seed.APIEndpoint)
	if configExists && providerConfig.BaseURL != "" && baseURLOverride == "" {
		resolved, err := resolver.ResolveValue(providerConfig.BaseURL)
		if err != nil {
			return nil, false, fmt.Errorf("failed to resolve Copilot base URL: %w", err)
		}
		baseURL = resolved
	}

	apiKey := ""
	if configExists && providerConfig.APIKey != "" {
		resolved, err := resolver.ResolveValue(providerConfig.APIKey)
		if err != nil {
			return nil, false, fmt.Errorf("failed to resolve Copilot API key: %w", err)
		}
		apiKey = strings.TrimSpace(resolved)
	}
	oauthToken := providerConfig.OAuthToken
	credentialed := apiKey != "" || usableOAuthToken(oauthToken)

	return &realCopilotModelsClient{baseURL: baseURL, apiKey: apiKey, oauthToken: oauthToken}, credentialed, nil
}

func usableOAuthToken(token *oauth.Token) bool {
	if token == nil {
		return false
	}
	return strings.TrimSpace(token.AccessToken) != "" || strings.TrimSpace(token.RefreshToken) != ""
}

func updateLiveProvider(name, cacheName string, providerID catwalk.InferenceProvider, pathOrURL string, cfg *Config, newClient liveProviderClientFunc) error {
	seed, err := liveProviderSeed(providerID)
	if err != nil {
		return err
	}

	var provider catwalk.Provider
	switch {
	case pathOrURL == "embedded":
		provider = seed
	case pathOrURL != "" && !strings.HasPrefix(pathOrURL, "http://") && !strings.HasPrefix(pathOrURL, "https://"):
		content, err := os.ReadFile(pathOrURL)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if err := json.Unmarshal(content, &provider); err != nil {
			return fmt.Errorf("failed to unmarshal provider data: %w", err)
		}
	default:
		if cfg == nil {
			return fmt.Errorf("failed to fetch provider from %s: %w", name, errMissingLiveProviderCredentials)
		}
		environment := env.New()
		resolver := NewShellVariableResolver(environment)
		client, credentialed, err := newClient(seed, cfg, resolver, pathOrURL)
		if err != nil {
			return fmt.Errorf("failed to prepare %s provider update: %w", name, err)
		}
		if !credentialed {
			return fmt.Errorf("failed to fetch provider from %s: %w", name, errMissingLiveProviderCredentials)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		provider, err = client.Get(ctx, "")
		if err != nil {
			return fmt.Errorf("failed to fetch provider from %s: %w", name, err)
		}
		if len(provider.Models) == 0 {
			return fmt.Errorf("failed to fetch provider from %s: no models returned", name)
		}
		provider = mergeLiveProvider(seed, provider)
	}

	if err := newCache[catwalk.Provider](cachePathFor(cacheName)).Store(provider); err != nil {
		return fmt.Errorf("failed to save %s provider to cache: %w", name, err)
	}

	slog.Info(name+" provider updated successfully", "from", cmp.Or(pathOrURL, "live"), "to", cachePathFor(cacheName))
	return nil
}

func liveProviderSeed(providerID catwalk.InferenceProvider) (catwalk.Provider, error) {
	providers, _, err := newCache[[]catwalk.Provider](cachePathFor("providers")).Get()
	if err != nil || len(providers) == 0 {
		providers = embedded.GetAll()
	}
	if provider, ok := findProvider(providers, providerID); ok {
		return provider, nil
	}
	if provider, ok := findProvider(embedded.GetAll(), providerID); ok {
		return provider, nil
	}
	return catwalk.Provider{}, fmt.Errorf("provider %s not found", providerID)
}

func findProvider(providers []catwalk.Provider, providerID catwalk.InferenceProvider) (catwalk.Provider, bool) {
	for _, provider := range providers {
		if provider.ID == providerID {
			return provider, true
		}
	}
	return catwalk.Provider{}, false
}

type cache[T any] struct {
	path string
}

func newCache[T any](path string) cache[T] {
	return cache[T]{path: path}
}

func (c cache[T]) Get() (T, string, error) {
	var v T
	data, err := os.ReadFile(c.path)
	if err != nil {
		return v, "", fmt.Errorf("failed to read provider cache file: %w", err)
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return v, "", fmt.Errorf("failed to unmarshal provider data from cache: %w", err)
	}

	return v, etag.Of(data), nil
}

func (c cache[T]) Store(v T) error {
	slog.Info("Saving provider data to disk", "path", c.path)
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for provider cache: %w", err)
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write provider data to cache: %w", err)
	}
	return nil
}
