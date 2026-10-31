package config

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"sync"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/grok"
	"github.com/charmbracelet/crush/internal/oauth/openai"
)

// subscription describes a provider whose OAuth login grants its own model
// catalog, stored in ProviderConfig.SubscriptionModels. Tests stub fetch
// by swapping the entry in subscriptions.
type subscription struct {
	fetch func(context.Context, *oauth.Token) ([]catwalk.Model, error)
	// keyRetiresLogin is false when the API key only mirrors the login's
	// short-lived token, so the login must stay to keep refreshing it.
	keyRetiresLogin bool
}

var subscriptions = map[string]subscription{
	string(catwalk.InferenceProviderOpenAI):  {fetch: openai.Models, keyRetiresLogin: true},
	string(catwalk.InferenceProviderXAI):     {fetch: grok.Models, keyRetiresLogin: true},
	string(catwalk.InferenceProviderCopilot): {fetch: copilot.Models},
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

// SignedInModels returns the subscription catalog while signed in to one,
// or nil.
func (p ProviderConfig) SignedInModels() []catwalk.Model {
	if p.OAuthToken == nil || len(p.SubscriptionModels) == 0 {
		return nil
	}
	return p.SubscriptionModels
}

// ActiveModels returns the catalog the provider's current credential can
// use: the subscription's while signed in to one, the static one otherwise.
func (p ProviderConfig) ActiveModels() []catwalk.Model {
	if models := p.SignedInModels(); models != nil {
		return models
	}
	return p.Models
}

// findModel returns a copy of the model with the given ID from either of
// the provider's catalogs, or nil.
func (p ProviderConfig) findModel(id string) *catwalk.Model {
	for _, models := range [][]catwalk.Model{p.Models, p.SubscriptionModels} {
		for _, m := range models {
			if m.ID == id {
				return &m
			}
		}
	}
	return nil
}

// credentialIdentity names the login a token belongs to. Access tokens
// rotate on every refresh, so they cannot tell two logins apart.
func credentialIdentity(t *oauth.Token) string {
	return cmp.Or(t.AccountID, t.RefreshToken, t.AccessToken)
}

// refetchSubscriptionModels stores the catalog the provider's login
// grants. Best effort: a failure leaves the existing catalog in place.
func (s *ConfigStore) refetchSubscriptionModels(ctx context.Context, scope Scope, providerID string) {
	if !HasSubscriptionCatalog(providerID) {
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
	models, err := subscriptions[providerID].fetch(ctx, pc.OAuthToken)
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
		p.SubscriptionModels = models
		c.Providers.Set(providerID, p)
		return map[string]any{subscriptionModelsKey(providerID): models}
	}); err != nil {
		slog.Warn("Failed to persist model catalog", "provider", providerID, "error", err)
	}
}

// refetchSignedInSubscriptions concurrently refetches the catalog of every
// signed-in subscription provider, or only those missing one.
func (s *ConfigStore) refetchSignedInSubscriptions(ctx context.Context, onlyMissing bool) {
	var wg sync.WaitGroup
	for providerID := range subscriptions {
		pc, ok := s.Config().Providers.Get(providerID)
		if !ok || pc.OAuthToken == nil || (onlyMissing && len(pc.SubscriptionModels) > 0) {
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
	s.refetchSignedInSubscriptions(ctx, true)
}
