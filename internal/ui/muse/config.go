// Package muse implements background thinking during user inactivity.
package muse

import (
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

// Default configuration values.
const (
	DefaultInterval  = 300 * time.Second
	TickInterval      = 1 * time.Second
	DefaultContinuity = false // trigger once by default
)

// DefaultPrompt is the default Muse prompt.
const DefaultPrompt = "Review the current file context and recent changes. Briefly identify any potential edge cases, logic gaps, or refactoring opportunities. If the code looks solid, suggest a single, high-impact next step or a relevant test case. Keep your response concise, friendly, and non-intrusive. Avoid repeating what is already obvious."

// Config returns Muse configuration from app config.
type Config struct {
	Enabled    bool
	Interval   time.Duration
	Continuity bool // if true, keep triggering every interval
	Prompt     string
}

// GetConfig extracts Muse configuration from app config.
func GetConfig(cfg *config.Config) Config {
	c := Config{
		Enabled:    false, // Always starts disabled - toggle at runtime
		Interval:   DefaultInterval,
		Continuity: DefaultContinuity,
		Prompt:     DefaultPrompt,
	}

	if cfg.Options == nil {
		return c
	}

	if cfg.Options.MuseInterval > 0 {
		c.Interval = time.Duration(cfg.Options.MuseInterval) * time.Second
	}
	if cfg.Options.MuseContinuity {
		c.Continuity = cfg.Options.MuseContinuity
	}
	if cfg.Options.MusePrompt != "" {
		c.Prompt = cfg.Options.MusePrompt
	}

	return c
}
