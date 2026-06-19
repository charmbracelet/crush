package discover

import (
	"context"
	"sort"

	"charm.land/catwalk/pkg/catwalk"
)

// Enricher fills in model metadata (context window, max tokens, pricing,
// etc.) for discovered models. Providers that expose richer endpoints
// beyond /v1/models can register an Enricher to populate fields that the
// standard listing endpoint omits.
type Enricher interface {
	// EnrichModels takes a slice of bare discovered models and returns
	// them with metadata populated. Implementations should preserve
	// existing non-zero fields (user overrides take precedence).
	EnrichModels(ctx context.Context, cfg Config, resolver Resolver, models []catwalk.Model) ([]catwalk.Model, error)
}

// enrichers maps provider type strings to their enrichment
// implementations. Each enricher self-registers via init() so that
// adding a new provider requires only a new file — no changes to
// load.go or any existing enricher.
var enrichers = map[string]Enricher{}

// RegisterEnricher registers an Enricher for the given provider type.
// Called from init() in each enricher implementation file.
func RegisterEnricher(providerType string, e Enricher) {
	enrichers[providerType] = e
}

// GetEnricher returns the Enricher for the given provider type, or nil
// if no enricher is registered.
func GetEnricher(providerType string) Enricher {
	return enrichers[providerType]
}

// IsKnownCustomProvider reports whether the given provider type has a
// registered enricher. This signals that the provider type is
// recognized and speaks OpenAI-compat protocol, without exposing the
// enricher itself to callers that only need the compatibility check.
func IsKnownCustomProvider(providerType string) bool {
	return enrichers[providerType] != nil
}

// RegisteredProviderTypes returns the provider type strings that have a
// registered enricher, sorted for stable output. These are the custom,
// locally-discovered providers (e.g. ollama, omlx) that Crush accepts as
// a `type` value even though they are not catwalk provider types. The
// schema generator uses this so the published enum stays in sync with the
// registry instead of drifting from a hand-maintained list.
func RegisteredProviderTypes() []string {
	types := make([]string, 0, len(enrichers))
	for providerType := range enrichers {
		types = append(types, providerType)
	}
	sort.Strings(types)
	return types
}
