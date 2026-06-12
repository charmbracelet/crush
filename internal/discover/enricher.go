package discover

import (
	"context"

	"charm.land/catwalk/pkg/catwalk"
)

// Enricher fills in metadata (context window, max tokens, pricing) for
// discovered models by hitting provider-specific endpoints.
type Enricher interface {
	// EnrichModels populates metadata on the given models. It may also
	// append new models that were not returned by the basic /v1/models
	// endpoint. The returned slice replaces the input.
	EnrichModels(ctx context.Context, cfg Config, models []catwalk.Model) ([]catwalk.Model, error)
}

var enrichers map[string]Enricher

func init() {
	enrichers = make(map[string]Enricher)
}

// RegisterEnricher registers an enricher for the given provider type.
// Provider types are case-sensitive (e.g. "litellm", "ollama").
func RegisterEnricher(providerType string, e Enricher) {
	enrichers[providerType] = e
}

// GetEnricher returns the enricher registered for the given provider
// type, or nil if none is registered.
func GetEnricher(providerType string) Enricher {
	return enrichers[providerType]
}

// IsKnownCustomProvider returns true if the given provider type has an
// enricher registered or is otherwise known to the discover package.
func IsKnownCustomProvider(providerType string) bool {
	_, ok := enrichers[providerType]
	return ok
}
