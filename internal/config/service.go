package config

import "charm.land/catwalk/pkg/catwalk"

// Service is the central access point for configuration. It wraps the
// raw Config data and owns all internal state that was previously held
// as unexported fields on Config (resolver, store, known providers,
// working directory).
type Service struct {
	cfg            *Config
	store          Store
	resolver       VariableResolver
	workingDir     string
	knownProviders []catwalk.Provider
}

// Config returns the underlying Config struct. This is a temporary
// escape hatch that will be removed once all callers migrate to
// Service getter methods.
func (s *Service) Config() *Config {
	return s.cfg
}
