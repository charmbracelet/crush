package config

import (
	"cmp"
	"context"
	"log/slog"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/discover"
)

// discoveryJob describes one custom provider whose models should be
// discovered from its /models endpoint.
type discoveryJob struct {
	id           string
	cfg          discover.Config
	providerType catwalk.Type
}

// collectDiscoveryJobs returns the custom providers that need model
// discovery: enabled, with a base URL, and either opted in via
// discover_models or lacking an explicit models list. known holds the ids
// of bundled providers, which never participate in discovery.
func collectDiscoveryJobs(cfg *Config, known map[string]bool) []discoveryJob {
	var jobs []discoveryJob
	for id, pc := range cfg.Providers.Seq2() {
		if known[id] || pc.Disable || pc.BaseURL == "" {
			continue
		}
		wantsDiscovery := pc.AutoDiscoverModels != nil && *pc.AutoDiscoverModels
		autoTrigger := len(pc.Models) == 0 && (pc.AutoDiscoverModels == nil || *pc.AutoDiscoverModels)
		if !wantsDiscovery && !autoTrigger {
			continue
		}
		jobs = append(jobs, discoveryJob{
			id: id,
			cfg: discover.Config{
				ID:             cmp.Or(pc.ID, id),
				BaseURL:        pc.BaseURL,
				APIKey:         pc.APIKey,
				ExtraHeaders:   pc.ExtraHeaders,
				ExistingModels: pc.Models,
			},
			providerType: cmp.Or(pc.Type, catwalk.TypeOpenAICompat),
		})
	}
	return jobs
}

// setPendingDiscovery records the providers awaiting model discovery.
// Called from configureProviders while all providers are still present,
// before empty ones are dropped.
func (s *ConfigStore) setPendingDiscovery(jobs []discoveryJob) {
	s.discoveryMu.Lock()
	defer s.discoveryMu.Unlock()
	s.pendingDiscovery = jobs
}

func (s *ConfigStore) pendingDiscoveryJobs() []discoveryJob {
	s.discoveryMu.Lock()
	defer s.discoveryMu.Unlock()
	return s.pendingDiscovery
}

// isPendingDiscovery reports whether the given provider id is still
// awaiting background model discovery. Used to avoid persisting a
// model-selection fallback for a provider whose models simply have not
// been discovered yet.
func (s *ConfigStore) isPendingDiscovery(providerID string) bool {
	if providerID == "" {
		return false
	}
	for _, job := range s.pendingDiscoveryJobs() {
		if job.id == providerID {
			return true
		}
	}
	return false
}

// HasPendingModelDiscovery reports whether any custom provider still needs
// its models discovered. Callers use this to decide whether to show a
// discovery indicator before kicking off a background pass.
func (s *ConfigStore) HasPendingModelDiscovery() bool {
	return len(s.pendingDiscoveryJobs()) > 0
}

// DiscoverModels fetches models for every custom provider that needs them,
// stores the results in the in-memory cache, and reloads the config so the
// discovered models take effect. It returns whether any new models were
// found. Intended to run in the background: it makes network calls and
// respects the caller's context for cancellation.
func (s *ConfigStore) DiscoverModels(ctx context.Context) (bool, error) {
	jobs := s.pendingDiscoveryJobs()
	if len(jobs) == 0 {
		return false, nil
	}

	if s.discoveredModels == nil {
		s.discoveredModels = csync.NewMap[string, []catwalk.Model]()
	}

	resolver := s.resolver
	var wg sync.WaitGroup
	var mu sync.Mutex
	changed := false

	for _, job := range jobs {
		wg.Go(func() {
			models, err := discover.DiscoverModels(ctx, job.cfg, resolver)
			if err != nil {
				slog.Debug("Background model discovery failed", "provider", job.id, "error", err)
				return
			}
			if len(models) == 0 {
				return
			}
			if enricher := discover.GetEnricher(string(job.providerType)); enricher != nil {
				models, _ = enricher.EnrichModels(ctx, job.cfg, resolver, models)
			}
			mu.Lock()
			s.discoveredModels.Set(job.id, models)
			changed = true
			mu.Unlock()
			slog.Info("Discovered models for provider", "provider", job.id, "count", len(models))
		})
	}
	wg.Wait()

	if !changed {
		return false, nil
	}
	if err := s.ReloadFromDisk(ctx); err != nil {
		return true, err
	}
	return true, nil
}
