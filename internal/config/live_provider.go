package config

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

const (
	liveModelsTTL            = time.Minute
	liveProviderFetchTimeout = 45 * time.Second
)

type liveProviderClient interface {
	Get(context.Context) (catwalk.Provider, error)
}

var _ syncer[catwalk.Provider] = (*liveProviderSync)(nil)

type liveProviderSync struct {
	once         sync.Once
	resultMu     sync.RWMutex
	result       catwalk.Provider
	cache        cache[catwalk.Provider]
	client       liveProviderClient
	onRefresh    func(catwalk.Provider)
	seed         catwalk.Provider
	autoupdate   bool
	credentialed bool
	ttl          time.Duration
	init         atomic.Bool
}

func (s *liveProviderSync) Init(client liveProviderClient, path string, autoupdate bool, seed catwalk.Provider, credentialed bool) {
	s.client = client
	s.cache = newCache[catwalk.Provider](path)
	s.autoupdate = autoupdate
	s.seed = seed
	s.credentialed = credentialed
	s.ttl = liveModelsTTL
	s.init.Store(true)
}

func (s *liveProviderSync) Get(ctx context.Context) (catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Get before Init")
	}

	s.once.Do(func() {
		if !s.autoupdate {
			slog.Info("Using provider seed", "provider", s.seed.ID)
			s.setResult(s.seed)
			return
		}
		if !s.credentialed {
			slog.Info("Skipping live provider sync without credentials", "provider", s.seed.ID)
			s.setResult(s.seed)
			return
		}

		cached, _, cachedErr := s.cache.Get()
		cachedAvailable := cachedErr == nil && len(cached.Models) > 0
		fallback := s.seed
		if cachedAvailable {
			fallback = cached
		}

		if cachedAvailable {
			if age, ok := cacheAge(s.cache.path); ok && age < s.ttl {
				slog.Info("Using cached live provider models", "provider", fallback.ID, "age", age)
				s.setResult(cached)
				return
			}
		}

		s.setResult(fallback)
		s.refreshInBackground()
	})
	return s.getResult(), nil
}

func (s *liveProviderSync) refreshInBackground() {
	slog.Info("Refreshing live provider models in background", "provider", s.seed.ID)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), liveProviderFetchTimeout)
		defer cancel()

		result, err := s.client.Get(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			slog.Warn("Live provider models not updated in time", "provider", s.seed.ID)
			return
		}
		if err != nil {
			slog.Warn("Live provider models not updated", "provider", s.seed.ID, "err", err)
			return
		}
		if len(result.Models) == 0 {
			slog.Warn("Live provider did not return any models", "provider", s.seed.ID)
			return
		}

		merged := mergeLiveProvider(s.seed, result)
		s.setResult(merged)
		if err := s.cache.Store(merged); err != nil {
			slog.Warn("Failed to store live provider cache", "provider", s.seed.ID, "err", err)
			return
		}
		if s.onRefresh != nil {
			s.onRefresh(merged)
		}
	}()
}

func (s *liveProviderSync) setResult(provider catwalk.Provider) {
	s.resultMu.Lock()
	defer s.resultMu.Unlock()
	s.result = provider
}

func (s *liveProviderSync) getResult() catwalk.Provider {
	s.resultMu.RLock()
	defer s.resultMu.RUnlock()
	return s.result
}

func cacheAge(path string) (time.Duration, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0, false
	}

	age := max(time.Since(info.ModTime()), 0)
	return age, true
}

func mergeLiveProvider(seed, live catwalk.Provider) catwalk.Provider {
	merged := seed
	if live.ID != "" {
		merged.ID = live.ID
	}
	if live.Name != "" {
		merged.Name = live.Name
	}
	if live.APIKey != "" {
		merged.APIKey = live.APIKey
	}
	if live.APIEndpoint != "" {
		merged.APIEndpoint = live.APIEndpoint
	}
	if live.Type != "" {
		merged.Type = live.Type
	}
	if live.DefaultLargeModelID != "" {
		merged.DefaultLargeModelID = live.DefaultLargeModelID
	}
	if live.DefaultSmallModelID != "" {
		merged.DefaultSmallModelID = live.DefaultSmallModelID
	}
	if len(live.DefaultHeaders) > 0 {
		merged.DefaultHeaders = live.DefaultHeaders
	}
	merged.Models = live.Models

	if len(merged.Models) == 0 {
		return merged
	}

	if merged.DefaultLargeModelID == "" || !modelExists(merged.Models, merged.DefaultLargeModelID) {
		merged.DefaultLargeModelID = merged.Models[0].ID
	}
	if merged.DefaultSmallModelID == "" || !modelExists(merged.Models, merged.DefaultSmallModelID) {
		merged.DefaultSmallModelID = merged.Models[0].ID
	}
	return merged
}

func modelExists(models []catwalk.Model, id string) bool {
	return slices.ContainsFunc(models, func(model catwalk.Model) bool {
		return model.ID == id
	})
}
