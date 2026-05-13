package config

import (
	"sync"

	"github.com/taigrr/catwalk/pkg/catwalk"
	"github.com/taigrr/catwalk/pkg/embedded"
)

var (
	providerOnce sync.Once
	providerList []catwalk.Provider
)

// UpdateProviders is a no-op kept for CLI compatibility.
func UpdateProviders(_ string) error {
	return nil
}

// UpdateHyper is a no-op kept for CLI compatibility.
func UpdateHyper(_ string) error {
	return nil
}

// Providers returns the compiled-in provider catalog. No network calls, no
// server dependency, no local files. Custom providers from config are merged
// by the caller.
func Providers(_ *Config) ([]catwalk.Provider, error) {
	providerOnce.Do(func() {
		providerList = embedded.GetAll()
	})
	return providerList, nil
}
