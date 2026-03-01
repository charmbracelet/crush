// Package muse implements background thinking during user inactivity.
package muse

import (
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

// Default configuration values.
const (
	DefaultTimeout  = 120 * time.Second
	DefaultInterval = 0 // trigger once
	TickInterval    = 1 * time.Second
)

// DefaultPrompt is the default Muse prompt.
const DefaultPrompt = "Take a quiet look at what I've been working on. Does anything look off? Any 'gotchas' I might have missed? Give me a quick, friendly tip or a 'looks great' before I get back to work."

// Config returns Muse configuration from the app config.
type Config struct {
	Enabled  bool
	Timeout  time.Duration
	Interval time.Duration
	Prompt   string
}

// GetConfig extracts Muse configuration from app config.
func GetConfig(cfg *config.Config) Config {
	c := Config{
		Enabled:  false, // Always starts disabled - toggle at runtime
		Timeout:  DefaultTimeout,
		Interval: DefaultInterval,
		Prompt:   DefaultPrompt,
	}

	if cfg.Options == nil {
		return c
	}

	if cfg.Options.MuseTimeout > 0 {
		c.Timeout = time.Duration(cfg.Options.MuseTimeout) * time.Second
	}
	if cfg.Options.MuseInterval > 0 {
		c.Interval = time.Duration(cfg.Options.MuseInterval) * time.Second
	}
	if cfg.Options.MusePrompt != "" {
		c.Prompt = cfg.Options.MusePrompt
	}

	return c
}
