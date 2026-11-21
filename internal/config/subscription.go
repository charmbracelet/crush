package config

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"unicode"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/grok"
	"github.com/charmbracelet/crush/internal/oauth/openai"
)

// subscription describes a provider whose OAuth login grants its own model
// catalog, stored in ProviderConfig.SubscriptionCatalog. Tests stub fetch
// by swapping the entry in subscriptions.
type subscription struct {
	fetch func(context.Context, *oauth.Token) ([]catwalk.Model, error)
	// keyRetiresLogin is false when the API key only mirrors the login's
	// short-lived token, so the login must stay to keep refreshing it.
	keyRetiresLogin bool
	// curated means the fetched catalog only says which models are enabled;
	// the static catalog's entries carry the metadata requests route on.
	curated bool
	// smallMarkers are whole ID segments marking a lightweight model, most
	// preferred first.
	smallMarkers []string
}

var subscriptions = map[string]subscription{
	string(catwalk.InferenceProviderOpenAI): {
		fetch:           openai.Models,
		keyRetiresLogin: true,
		smallMarkers:    []string{"mini"},
	},
	string(catwalk.InferenceProviderXAI): {
		fetch:           grok.Models,
		keyRetiresLogin: true,
		smallMarkers:    []string{"fast", "mini"},
	},
	string(catwalk.InferenceProviderCopilot): {
		fetch:        copilot.Models,
		curated:      true,
		smallMarkers: []string{"mini", "haiku", "flash"},
	},
}

// HasSubscriptionCatalog reports whether logging in to the provider swaps
// its model catalog for the one the subscription grants.
func HasSubscriptionCatalog(providerID string) bool {
	_, ok := subscriptions[providerID]
	return ok
}

func subscriptionModelsKey(providerID string) string {
	return fmt.Sprintf("providers.%s.subscription_models", providerID)
}

// SubscriptionModels returns the models a subscription login grants, and
// whether the provider is signed in with one at all. The models are nil
// until the catalog is fetched.
func (p ProviderConfig) SubscriptionModels() ([]catwalk.Model, bool) {
	sub, ok := subscriptions[p.ID]
	if !ok || p.OAuthToken == nil {
		return nil, false
	}
	if !sub.curated || p.SubscriptionCatalog == nil {
		return p.SubscriptionCatalog, true
	}
	models := make([]catwalk.Model, len(p.SubscriptionCatalog))
	for i, m := range p.SubscriptionCatalog {
		if idx := slices.IndexFunc(p.Models, func(s catwalk.Model) bool { return s.ID == m.ID }); idx >= 0 {
			m = p.Models[idx]
		}
		models[i] = m
	}
	return models, true
}

// UsableModels returns the models the current credential can serve: a
// subscription login only reaches the models it grants. Until that list is
// fetched, it falls back to the static catalog.
func (p ProviderConfig) UsableModels() []catwalk.Model {
	if models, _ := p.SubscriptionModels(); models != nil {
		return models
	}
	return p.Models
}

// SubscriptionSmallModel picks a lightweight model from a signed-in
// subscription's catalog: preferred (the provider's default small model)
// when the subscription grants it, else the first marker match, else the
// last entry (catalogs list heavier models first). Returns nil when there is
// no such catalog.
func (p ProviderConfig) SubscriptionSmallModel(preferred string) *catwalk.Model {
	models, _ := p.SubscriptionModels()
	if i := slices.IndexFunc(models, func(m catwalk.Model) bool { return m.ID == preferred }); i >= 0 {
		return &models[i]
	}
	for _, marker := range subscriptions[p.ID].smallMarkers {
		for i := range models {
			if slices.Contains(idSegments(models[i].ID), marker) {
				return &models[i]
			}
		}
	}
	if len(models) > 0 {
		return &models[len(models)-1]
	}
	return nil
}

// idSegments splits a model ID into its alphanumeric runs, so a marker like
// "mini" matches "gpt-5-mini" but not "gemini-2.5-pro".
func idSegments(id string) []string {
	return strings.FieldsFunc(id, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// findModel returns a copy of the model with the given ID from either of
// the provider's catalogs, or nil. The static entry wins, since it carries
// the curated metadata.
func (p ProviderConfig) findModel(id string) *catwalk.Model {
	for _, models := range [][]catwalk.Model{p.Models, p.SubscriptionCatalog} {
		for _, m := range models {
			if m.ID == id {
				return &m
			}
		}
	}
	return nil
}

// credentialIdentity names the login a token belongs to, to spot a fetch
// that outlived its login. It is a staleness check, not a security
// boundary.
func credentialIdentity(t *oauth.Token) string {
	return cmp.Or(t.AccountID, t.RefreshToken, t.AccessToken)
}

// refetchSubscriptionModels stores the catalog the provider's login
// grants. Best effort: a failure leaves the existing catalog in place.
func (s *ConfigStore) refetchSubscriptionModels(ctx context.Context, scope Scope, providerID string) {
	sub, ok := subscriptions[providerID]
	if !ok {
		return
	}
	pc, ok := s.Config().Providers.Get(providerID)
	if !ok || pc.OAuthToken == nil {
		return
	}
	// The models endpoints reject expired tokens with a 401.
	if pc.OAuthToken.IsExpired() {
		if err := s.RefreshOAuthToken(ctx, scope, providerID); err != nil {
			slog.Warn("Failed to refresh token before fetching the model catalog", "provider", providerID, "error", err)
			return
		}
		pc, ok = s.Config().Providers.Get(providerID)
		if !ok || pc.OAuthToken == nil {
			return
		}
	}
	identity := credentialIdentity(pc.OAuthToken)
	models, err := sub.fetch(ctx, pc.OAuthToken)
	if err != nil {
		slog.Warn("Failed to fetch model catalog", "provider", providerID, "error", err)
		return
	}
	if err := s.update(scope, func(c *Config) map[string]any {
		p, ok := c.Providers.Get(providerID)
		// Drop the result if the login changed while the fetch was in flight.
		if !ok || p.OAuthToken == nil || credentialIdentity(p.OAuthToken) != identity {
			return nil
		}
		p.SubscriptionCatalog = models
		c.Providers.Set(providerID, p)
		return map[string]any{subscriptionModelsKey(providerID): models}
	}); err != nil {
		slog.Warn("Failed to persist model catalog", "provider", providerID, "error", err)
	}
}

// refetchSignedInSubscriptions concurrently refetches the catalog of every
// signed-in subscription provider that want selects.
func (s *ConfigStore) refetchSignedInSubscriptions(ctx context.Context, want func(ProviderConfig) bool) {
	var wg sync.WaitGroup
	for providerID := range subscriptions {
		pc, ok := s.Config().Providers.Get(providerID)
		if !ok || pc.OAuthToken == nil || !want(pc) {
			continue
		}
		wg.Go(func() { s.refetchSubscriptionModels(ctx, ScopeGlobal, providerID) })
	}
	wg.Wait()
}

// RefetchSubscriptionModels fills in any signed-in provider's missing
// subscription catalog, e.g. when the fetch at login failed. Existing
// catalogs are refreshed at startup when Catwalk updates (see Load).
func (s *ConfigStore) RefetchSubscriptionModels(ctx context.Context) {
	s.refetchSignedInSubscriptions(ctx, func(pc ProviderConfig) bool {
		return len(pc.SubscriptionCatalog) == 0
	})
}
