package config

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/catwalk/pkg/embedded"
)

type catwalkClient interface {
	GetProviders(context.Context, string) ([]catwalk.Provider, error)
}

var _ syncer[[]catwalk.Provider] = (*catwalkSync)(nil)

type catwalkSync struct {
	once       sync.Once
	result     []catwalk.Provider
	err        error
	cache      cache[[]catwalk.Provider]
	client     catwalkClient
	autoupdate bool
	init       atomic.Bool
}

func (s *catwalkSync) Init(client catwalkClient, path string, autoupdate bool) {
	s.client = client
	s.cache = newCache[[]catwalk.Provider](path)
	s.autoupdate = autoupdate
	s.init.Store(true)
}

func (s *catwalkSync) Get(ctx context.Context) ([]catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Get before Init")
	}

	// The result and the error are memoized together so that every caller
	// sees the same outcome, not just the one that won the once.
	s.once.Do(func() {
		if !s.autoupdate {
			slog.Info("Using embedded Catwalk providers")
			s.result = embedded.GetAll()
			return
		}

		cached, etag, cachedErr := s.cache.Get()
		if len(cached) == 0 || cachedErr != nil {
			// if cached file is empty, default to embedded providers
			cached = embedded.GetAll()
		}

		slog.Info("Fetching providers from Catwalk")
		result, err := s.client.GetProviders(ctx, etag)
		if errors.Is(err, context.DeadlineExceeded) {
			slog.Warn("Catwalk providers not updated in time")
			s.result = cached
			return
		}
		if errors.Is(err, catwalk.ErrNotModified) {
			slog.Info("Catwalk providers not modified")
			s.result = cached
			return
		}
		if err != nil {
			// Fall back to cached (which defaults to embedded if empty).
			// Being offline is routine and the fallback is sound, so this
			// is logged rather than reported to the caller.
			slog.Warn("Could not fetch providers from Catwalk", "error", err)
			s.result = cached
			return
		}
		if len(result) == 0 {
			s.result = cached
			s.err = errors.New("empty providers list from catwalk")
			return
		}

		// The catalog is usable from here on. A cache write failure only
		// costs the next run a refresh, so it is reported alongside a valid
		// result rather than in place of one.
		s.result = result
		s.err = s.cache.Store(result)
	})
	return s.result, s.err
}
