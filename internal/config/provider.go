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
	"strings"
	"sync"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/catwalk/v3/pkg/embedded"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/x/etag"
)

// providerOnce initializes the syncers from the first config to ask.
var providerOnce sync.Once

// file to cache provider data. It resolves through GlobalConfigData so the
// catalog follows CRUSH_GLOBAL_DATA like the rest of the data directory.
func cachePathFor(name string) string {
	return filepath.Join(filepath.Dir(GlobalConfigData()), name+".json")
}

// UpdateProviders updates the Catwalk providers list from a specified source.
func UpdateProviders(pathOrURL string) error {
	var data []byte
	pathOrURL = cmp.Or(pathOrURL, os.Getenv("CATWALK_URL"), defaultCatwalkURL)

	switch {
	case pathOrURL == "embedded":
		var err error
		if data, err = json.Marshal(embedded.GetAll()); err != nil {
			return fmt.Errorf("failed to marshal embedded providers: %w", err)
		}
	case strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://"):
		var err error
		_, data, err = newCatwalkClient(pathOrURL).GetProviders(context.Background(), "")
		if err != nil {
			return fmt.Errorf("failed to fetch providers from Catwalk: %w", err)
		}
	default:
		var err error
		if data, err = os.ReadFile(pathOrURL); err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		var providers []catwalk.Provider
		if err := json.Unmarshal(data, &providers); err != nil {
			return fmt.Errorf("failed to unmarshal provider data: %w", err)
		}
		if len(providers) == 0 {
			return fmt.Errorf("no providers found in the provided source")
		}
	}

	if err := newCache[[]catwalk.Provider](cachePathFor("providers")).StoreRaw(data); err != nil {
		return fmt.Errorf("failed to save providers to cache: %w", err)
	}

	slog.Info("Providers updated successfully", "from", pathOrURL, "to", cachePathFor)
	return nil
}

// ResolveHyperAPIKey returns the Hyper API key from the environment or
// the raw config value. The env var takes precedence.
func ResolveHyperAPIKey(cfg *Config) string {
	if key := os.Getenv("HYPER_API_KEY"); key != "" {
		return key
	}
	if cfg == nil || cfg.Providers == nil {
		return ""
	}
	pc, ok := cfg.Providers.Get("hyper")
	if !ok {
		return ""
	}
	return pc.APIKey
}

// HyperTokenRefresher is a function that refreshes the Hyper OAuth
// token. It is passed to Providers so the catalog fetch can retry on
// 401 without relying on package-global state.
type HyperTokenRefresher func(context.Context) error

// UpdateHyper updates the Hyper provider information from a specified URL.
func UpdateHyper(pathOrURL string) error {
	var data []byte
	pathOrURL = cmp.Or(pathOrURL, hyper.BaseURL())

	switch {
	case pathOrURL == "embedded":
		var err error
		if data, err = json.Marshal(hyper.Embedded()); err != nil {
			return fmt.Errorf("failed to marshal embedded Hyper provider: %w", err)
		}
	case strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://"):
		client := realHyperClient{
			baseURL:    pathOrURL,
			resolveKey: func() string { return ResolveHyperAPIKey(nil) },
		}
		var err error
		if _, data, err = client.Get(context.Background(), ""); err != nil {
			return fmt.Errorf("failed to fetch provider from Hyper: %w", err)
		}
	default:
		var err error
		if data, err = os.ReadFile(pathOrURL); err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		var provider catwalk.Provider
		if err := json.Unmarshal(data, &provider); err != nil {
			return fmt.Errorf("failed to unmarshal provider data: %w", err)
		}
	}

	if err := newCache[catwalk.Provider](cachePathFor("hyper")).StoreRaw(data); err != nil {
		return fmt.Errorf("failed to save Hyper provider to cache: %w", err)
	}

	slog.Info("Hyper provider updated successfully", "from", pathOrURL, "to", cachePathFor("hyper"))
	return nil
}

var (
	catwalkSyncer = newCatwalkSync()
	hyperSyncer   = newHyperSync()
)

// Providers returns Hyper's then Catwalk's providers, cached or embedded. An
// error is advisory: the list is always usable.
func Providers(cfg *Config) ([]catwalk.Provider, error) {
	return LoadProviders(context.Background(), cfg, nil)
}

// LoadProviders is Providers with a cold-start deadline and a Hyper token
// refresher; the first call configures the syncers for the process.
func LoadProviders(ctx context.Context, cfg *Config, refreshHyper HyperTokenRefresher) ([]catwalk.Provider, error) {
	providerOnce.Do(func() {
		if cfg.Options.DisableDefaultProviders {
			return
		}
		autoupdate := !cfg.Options.DisableProviderAutoUpdate
		catwalkSyncer.Init(newCatwalkClient(catwalkURL()).GetProviders, cachePathFor("providers"), autoupdate)
		hyperSyncer.Init(realHyperClient{
			baseURL:      hyper.BaseURL(),
			resolveKey:   func() string { return ResolveHyperAPIKey(cfg) },
			refreshToken: refreshHyper,
		}.Get, cachePathFor("hyper"), autoupdate)
	})

	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var (
		catwalkProviders     []catwalk.Provider
		hyperProvider        catwalk.Provider
		catwalkErr, hyperErr error
		wg                   sync.WaitGroup
	)
	if catwalkSyncer.Initialized() {
		wg.Go(func() { catwalkProviders, catwalkErr = catwalkSyncer.Get(ctx) })
	}
	if hyperSyncer.Initialized() {
		wg.Go(func() { hyperProvider, hyperErr = hyperSyncer.Get(ctx) })
	}
	wg.Wait()

	if catwalkErr != nil {
		catwalkErr = fmt.Errorf("Crush was unable to fetch an updated list of providers from %s/v3/providers. Consider setting CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1 to use the embedded providers bundled at the time of this Crush release. You can also update providers manually. For more info see crush update-providers --help.\n\nCause: %w", catwalkURL(), catwalkErr) //nolint:staticcheck
	}
	if hyperErr != nil {
		hyperErr = fmt.Errorf("Crush was unable to fetch updated information from Hyper: %w", hyperErr) //nolint:staticcheck
	}
	if hyperProvider.ID == "" {
		return catwalkProviders, errors.Join(catwalkErr, hyperErr)
	}
	return append([]catwalk.Provider{hyperProvider}, catwalkProviders...), errors.Join(catwalkErr, hyperErr)
}

func catwalkURL() string { return cmp.Or(os.Getenv("CATWALK_URL"), defaultCatwalkURL) }

// providerGens identifies the Catwalk and Hyper catalog versions in use.
type providerGens [2]int

// currentProviderGens reads the versions before reading the catalogs, so a
// concurrent update can only make a store reload once more, never miss one.
func currentProviderGens() providerGens {
	return providerGens{catwalkSyncer.generation(), hyperSyncer.generation()}
}

// refreshProviders revalidates both catalogs, reporting which changed since
// the last call. Each change is reported once, so callers act on it once.
func refreshProviders(ctx context.Context) (catwalkChanged, hyperChanged bool, err error) {
	var catwalkErr, hyperErr error
	var wg sync.WaitGroup
	wg.Go(func() { catwalkChanged, catwalkErr = catwalkSyncer.Revalidate(ctx) })
	wg.Go(func() { hyperChanged, hyperErr = hyperSyncer.Revalidate(ctx) })
	wg.Wait()
	return catwalkChanged, hyperChanged, errors.Join(catwalkErr, hyperErr)
}

type cache[T any] struct {
	path string
}

func newCache[T any](path string) cache[T] {
	return cache[T]{path: path}
}

func (c cache[T]) Get() (T, string, error) {
	var v T
	data, err := readFile(c.path)
	if err != nil {
		return v, "", fmt.Errorf("failed to read provider cache file: %w", err)
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return v, "", fmt.Errorf("failed to unmarshal provider data from cache: %w", err)
	}

	return v, etag.Of(data), nil
}

func (c cache[T]) Store(v T) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}
	return c.StoreRaw(data)
}

// StoreRaw caches data exactly as given, so that Get derives the same ETag
// the server computed for it.
func (c cache[T]) StoreRaw(data []byte) error {
	slog.Info("Saving provider data to disk", "path", c.path)
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for provider cache: %w", err)
	}

	// Written through a temporary file and renamed into place. Several Crush
	// instances start independently and race to refresh this cache, and a
	// truncating write would let one of them read a half-written catalog and
	// silently fall back to the bundled copy.
	if err := atomicWriteFile(c.path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write provider data to cache: %w", err)
	}
	return nil
}
