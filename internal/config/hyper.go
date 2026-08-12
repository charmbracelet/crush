package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	xetag "github.com/charmbracelet/x/etag"
)

type hyperClient interface {
	Get(context.Context, string) (catwalk.Provider, error)
}

var _ syncer[catwalk.Provider] = (*hyperSync)(nil)

type hyperSync struct {
	once       sync.Once
	result     catwalk.Provider
	err        error
	cache      cache[catwalk.Provider]
	client     hyperClient
	autoupdate bool
	init       atomic.Bool
}

func (s *hyperSync) Init(client hyperClient, path string, autoupdate bool) {
	s.client = client
	s.cache = newCache[catwalk.Provider](path)
	s.autoupdate = autoupdate
	s.init.Store(true)
}

// SetClient replaces the HTTP client used for fetching. This is used
// before Refetch to ensure the latest credentials are used.
func (s *hyperSync) SetClient(client hyperClient) {
	s.client = client
}

func (s *hyperSync) Get(ctx context.Context) (catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Get before Init")
	}

	// The result and the error are memoized together so that every caller
	// sees the same outcome, not just the one that won the once.
	s.once.Do(func() {
		s.fetch(ctx)
	})
	return s.result, s.err
}

// Refetch resets the memoized result and re-fetches the Hyper provider.
// Must not be called concurrently with Get.
func (s *hyperSync) Refetch(ctx context.Context) (catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Refetch before Init")
	}
	s.once = sync.Once{}
	s.once.Do(func() {
		s.fetch(ctx)
	})
	return s.result, s.err
}

// fetch performs the actual Hyper provider fetch. It is called from both
// Get (via sync.Once) and Refetch (after resetting sync.Once).
func (s *hyperSync) fetch(ctx context.Context) {
	s.err = nil
	if !s.autoupdate {
		slog.Info("Using embedded Hyper provider")
		s.result = hyper.Embedded()
		return
	}

	cached, etag, cachedErr := s.cache.Get()
	if cached.ID == "" || cachedErr != nil {
		// if cached file is empty, default to embedded provider
		cached = hyper.Embedded()
	}

	slog.Info("Fetching Hyper provider")
	result, err := s.client.Get(ctx, etag)
	if errors.Is(err, context.DeadlineExceeded) {
		slog.Warn("Hyper provider not updated in time")
		s.result = cached
		return
	}
	if errors.Is(err, catwalk.ErrNotModified) {
		slog.Info("Hyper provider not modified")
		s.result = cached
		return
	}
	if err != nil {
		slog.Warn("Could not fetch the Hyper provider", "error", err)
		s.result = cached
		return
	}
	if len(result.Models) == 0 {
		slog.Warn("Hyper did not return any models")
		s.result = cached
		return
	}

	// The provider is usable from here on. A cache write failure only
	// costs the next run a refresh, so it is reported alongside a valid
	// result rather than in place of one.
	s.result = result
	s.err = s.cache.Store(result)
}

var _ hyperClient = realHyperClient{}

type realHyperClient struct {
	baseURL      string
	resolveKey   func() string
	refreshToken func(context.Context) error
}

// Get implements hyperClient.
func (r realHyperClient) Get(ctx context.Context, etag string) (catwalk.Provider, error) {
	result, err := r.doGet(ctx, etag)
	if err != nil && isHTTPUnauthorized(err) && r.refreshToken != nil {
		slog.Info("Received 401 fetching Hyper provider, refreshing token and retrying")
		if refreshErr := r.refreshToken(ctx); refreshErr != nil {
			slog.Warn("Failed to refresh Hyper token", "error", refreshErr)
			return result, err
		}
		result, err = r.doGet(ctx, "")
	}
	return result, err
}

func (r realHyperClient) doGet(ctx context.Context, etag string) (catwalk.Provider, error) {
	var result catwalk.Provider
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		r.baseURL+"/api/v1/provider",
		nil,
	)
	if err != nil {
		return result, fmt.Errorf("could not create request: %w", err)
	}
	xetag.Request(req, etag)
	if apiKey := r.resolveKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotModified {
		return result, catwalk.ErrNotModified
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return result, errUnauthorized
	}

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// errUnauthorized is a sentinel for HTTP 401 responses from the Hyper API.
var errUnauthorized = errors.New("unauthorized")

// isHTTPUnauthorized reports whether err is or wraps errUnauthorized.
func isHTTPUnauthorized(err error) bool {
	return errors.Is(err, errUnauthorized)
}
