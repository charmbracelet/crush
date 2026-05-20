package replacer

import (
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

const (
	DefaultMaxIterations = 3
	DefaultTimeout       = 30 * time.Second
)

// ReplacerConfig holds runtime configuration for the replacement agent.
type ReplacerConfig struct {
	Enabled       bool
	Model         string
	MaxIterations int
	Timeout       time.Duration
}

// NewReplacerConfig builds runtime config from the global config.
func NewReplacerConfig(cfg *config.Config) ReplacerConfig {
	if cfg == nil || cfg.Options == nil || cfg.Options.Replacer == nil {
		return ReplacerConfig{
			MaxIterations: DefaultMaxIterations,
			Timeout:       DefaultTimeout,
		}
	}

	rc := cfg.Options.Replacer
	c := ReplacerConfig{
		Enabled:       rc.IsEnabled(),
		Model:         rc.Model,
		MaxIterations: rc.MaxIterations,
		Timeout:       rc.Timeout,
	}
	if c.MaxIterations <= 0 {
		c.MaxIterations = DefaultMaxIterations
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	return c
}
