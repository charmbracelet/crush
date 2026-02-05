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

func (s *hyperSync) Get(ctx context.Context) (catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Get before Init")
	}

	var throwErr error
	s.once.Do(func() {
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
		if len(result.Models) == 0 {
			slog.Warn("Hyper did not return any models")
			s.result = cached
			return
		}

		s.result = result
		throwErr = s.cache.Store(result)
	})
	return s.result, throwErr
}

var _ hyperClient = realHyperClient{}

type realHyperClient struct {
	baseURL string
}

// Get implements hyperClient.
func (r realHyperClient) Get(ctx context.Context, etag string) (catwalk.Provider, error) {
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

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotModified {
		return result, catwalk.ErrNotModified
	}

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}
