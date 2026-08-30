// Package fantasyadapter isolates Fantasy SDK provider construction from the agent coordinator.
package fantasyadapter

import (
	"fmt"
	"net/http"
	"sync"

	"charm.land/fantasy"
)

// Request contains resolved runtime values needed to construct a Fantasy provider.
type Request struct {
	ProviderID string
	BaseURL    string
	APIKey     string
	Headers    map[string]string
	HTTPClient *http.Client
}

// Factory constructs one Fantasy provider implementation.
type Factory func(Request) (fantasy.Provider, error)

// Registry dispatches provider construction by provider type.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds a provider factory and rejects accidental replacement.
func (r *Registry) Register(providerType string, factory Factory) error {
	if providerType == "" || factory == nil {
		return fmt.Errorf("fantasy adapter: invalid provider registration")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[providerType]; exists {
		return fmt.Errorf("fantasy adapter: provider type %q already registered", providerType)
	}
	r.factories[providerType] = factory
	return nil
}

// Build constructs a registered provider.
func (r *Registry) Build(providerType string, request Request) (fantasy.Provider, error) {
	r.mu.RLock()
	factory, ok := r.factories[providerType]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("fantasy adapter: provider type %q is not registered", providerType)
	}
	return factory(request)
}

var defaultRegistry = newDefaultRegistry()

func newDefaultRegistry() *Registry {
	r := NewRegistry()
	if err := r.Register("openai", buildOpenAI); err != nil {
		panic(err)
	}
	if err := r.Register("anthropic", buildAnthropic); err != nil {
		panic(err)
	}
	return r
}

// Build constructs a provider using Ultra's compiled-in Fantasy adapters.
func Build(providerType string, request Request) (fantasy.Provider, error) {
	return defaultRegistry.Build(providerType, request)
}
