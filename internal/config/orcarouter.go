package config

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/orcarouter"
)

type orcaRouterClient interface {
	GetProviders(context.Context) ([]catwalk.Provider, error)
}

type realOrcaRouterClient struct{}

func (realOrcaRouterClient) GetProviders(ctx context.Context) ([]catwalk.Provider, error) {
	return orcarouter.FetchProviders(ctx)
}

var _ syncer[[]catwalk.Provider] = (*orcaRouterSync)(nil)

type orcaRouterSync struct {
	once       sync.Once
	result     []catwalk.Provider
	err        error
	cache      cache[[]catwalk.Provider]
	client     orcaRouterClient
	autoupdate bool
	init       atomic.Bool
}

func (s *orcaRouterSync) Init(client orcaRouterClient, path string, autoupdate bool) {
	s.client = client
	s.cache = newCache[[]catwalk.Provider](path)
	s.autoupdate = autoupdate
	s.init.Store(true)
}

func (s *orcaRouterSync) Get(ctx context.Context) ([]catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Get before Init")
	}

	s.once.Do(func() {
		fallback, fallbackErr := orcarouter.Providers()
		if fallbackErr != nil {
			s.err = fallbackErr
			return
		}
		if !s.autoupdate {
			slog.Info("Using embedded OrcaRouter providers")
			s.result = fallback
			return
		}

		cached, _, cachedErr := s.cache.Get()
		if len(cached) == 0 || cachedErr != nil {
			cached = fallback
		}
		result, err := s.client.GetProviders(ctx)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			slog.Warn("OrcaRouter providers not updated in time")
			s.result = cached
			return
		}
		if err != nil {
			slog.Warn("Could not fetch OrcaRouter providers", "error", err)
			s.result = cached
			return
		}
		if len(result) == 0 {
			s.result = cached
			s.err = errors.New("empty OrcaRouter providers list")
			return
		}
		s.result = result
		s.err = s.cache.Store(result)
	})
	return s.result, s.err
}
