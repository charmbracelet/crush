package config

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/minimax"
)

var _ syncer[catwalk.Provider] = (*minimaxSync)(nil)

type minimaxSync struct {
	once       sync.Once
	result     catwalk.Provider
	cache      cache[catwalk.Provider]
	autoupdate bool
	init       atomic.Bool
}

func (s *minimaxSync) Init(path string, autoupdate bool) {
	s.cache = newCache[catwalk.Provider](path)
	s.autoupdate = autoupdate
	s.init.Store(true)
}

func (s *minimaxSync) Get(ctx context.Context) (catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Get before Init")
	}

	var throwErr error
	s.once.Do(func() {
		if !s.autoupdate {
			slog.Info("Using embedded MiniMax provider")
			s.result = minimax.Embedded()
			return
		}

		cached, _, cachedErr := s.cache.Get()
		if cached.ID == "" || cachedErr != nil {
			// if cached file is empty, default to embedded provider
			cached = minimax.Embedded()
		}

		// For now, we just use the embedded provider
		// In the future, we could fetch from a remote source
		slog.Info("Using embedded MiniMax provider")
		s.result = minimax.Embedded()
		throwErr = s.cache.Store(s.result)
	})
	return s.result, throwErr
}
