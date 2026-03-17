package plugin

import (
	"context"
	"sync"
)

// SearchResult represents a single web search result.
type SearchResult struct {
	Title    string
	Link     string
	Snippet  string
	Position int
}

// SearchProvider performs web searches. Plugins can register a SearchProvider
// to replace the default DuckDuckGo scraper used by the agentic_fetch tool.
type SearchProvider interface {
	// Search executes a web search and returns results.
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
}

// SearchProviderFactory creates a SearchProvider during app initialization.
type SearchProviderFactory func(ctx context.Context, app *App) (SearchProvider, error)

// SearchProviderRegistration holds the factory and optional config schema.
type SearchProviderRegistration struct {
	Factory      SearchProviderFactory
	ConfigSchema any
}

var (
	searchProviderMu   sync.RWMutex
	searchProviderName string
	searchProviderReg  *SearchProviderRegistration
)

// RegisterSearchProvider registers a search provider plugin.
// Only one search provider can be registered. If multiple are registered,
// the last one wins. The provider replaces the built-in DuckDuckGo search.
func RegisterSearchProvider(name string, factory SearchProviderFactory) {
	RegisterSearchProviderWithConfig(name, factory, nil)
}

// RegisterSearchProviderWithConfig registers a search provider with config schema.
func RegisterSearchProviderWithConfig(name string, factory SearchProviderFactory, configSchema any) {
	searchProviderMu.Lock()
	defer searchProviderMu.Unlock()

	searchProviderName = name
	searchProviderReg = &SearchProviderRegistration{
		Factory:      factory,
		ConfigSchema: configSchema,
	}

	if configSchema != nil {
		configSchemas[name] = configSchema
	}
}

// GetSearchProviderRegistration returns the registered search provider, if any.
func GetSearchProviderRegistration() (string, *SearchProviderRegistration) {
	searchProviderMu.RLock()
	defer searchProviderMu.RUnlock()

	return searchProviderName, searchProviderReg
}

// ResetSearchProvider clears the registered search provider. Used for testing.
func ResetSearchProvider() {
	searchProviderMu.Lock()
	defer searchProviderMu.Unlock()

	searchProviderName = ""
	searchProviderReg = nil
}
