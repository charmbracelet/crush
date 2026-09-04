package oauth

import (
	"slices"
	"sync"
)

var (
	providersMu sync.RWMutex
	providers   = make(map[string]Provider)
)

// Register registers an OAuth provider.
func Register(p Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[p.ID()] = p
}

// Get returns the registered OAuth provider for the given ID, or nil if not registered.
func Get(id string) Provider {
	providersMu.RLock()
	defer providersMu.RUnlock()
	return providers[id]
}

// IsSupported reports whether the given provider ID has an OAuth implementation.
func IsSupported(id string) bool {
	return Get(id) != nil
}

// HasAuthChoices reports whether the provider offers both OAuth and API key auth.
func HasAuthChoices(id string) bool {
	p := Get(id)
	return p != nil && p.HasAuthChoices()
}

// All returns all registered OAuth providers sorted by ID.
func All() []Provider {
	providersMu.RLock()
	defer providersMu.RUnlock()
	list := make([]Provider, 0, len(providers))
	for _, p := range providers {
		list = append(list, p)
	}
	slices.SortFunc(list, func(a, b Provider) int {
		if a.ID() < b.ID() {
			return -1
		}
		if a.ID() > b.ID() {
			return 1
		}
		return 0
	})
	return list
}
