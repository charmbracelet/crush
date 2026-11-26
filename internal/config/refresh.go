package config

import (
	"context"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
)

// catalogRefreshTimeout bounds the network work of RefreshCatalogs.
const catalogRefreshTimeout = 30 * time.Second

// RefreshCatalogs updates every catalog startup took from a cache, and
// reloads the config if this store's are now out of date.
func (s *ConfigStore) RefreshCatalogs(ctx context.Context) (bool, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, catalogRefreshTimeout)
	defer cancel()

	var providersChanged, modelsChanged bool
	var wg sync.WaitGroup
	wg.Go(func() {
		catwalkNews, _, err := refreshProviders(fetchCtx)
		if err != nil {
			slog.Warn("Continuing with the previously known providers", "error", err)
		}
		// ChatGPT and Grok shadow Catwalk; one process-wide news report means one refetch.
		if catwalkNews {
			s.refetchOpenAIModels(fetchCtx, ScopeGlobal)
			s.refetchGrokModels(fetchCtx, ScopeGlobal)
		}
		s.writeMu.RLock()
		providersChanged = catwalkNews || s.providerGens != currentProviderGens()
		s.writeMu.RUnlock()
	})
	wg.Go(func() { modelsChanged = s.refreshDiscovered(fetchCtx) })
	wg.Wait()

	if !providersChanged && !modelsChanged {
		return false, nil
	}
	return true, s.ReloadFromDisk(ctx)
}

// refreshDiscovered re-probes the custom providers whose models came from
// the discovery cache, and reports whether any changed.
func (s *ConfigStore) refreshDiscovered(ctx context.Context) bool {
	s.writeMu.RLock()
	jobs := s.cachedDiscovery
	resolver := s.resolver
	s.writeMu.RUnlock()

	before := make(map[string][]catwalk.Model, len(jobs))
	for _, job := range jobs {
		before[job.id], _ = s.cachedModels(job)
	}
	changed := false
	for id, fresh := range s.probeAndCache(ctx, jobs, resolver) {
		changed = changed || !reflect.DeepEqual(before[id], fresh)
	}
	return changed
}
