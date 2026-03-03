package muse

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
)

// RunFunc is the function signature for running a Muse prompt.
// The context should be respected for cancellation.
type RunFunc func(ctx context.Context, sessionID, prompt string) error

// Muse manages background thinking during user inactivity.
// It is a pure state holder - all state mutations happen via methods
// that should only be called from Bubble Tea's Update() function.
type Muse struct {
	enabled bool
	interval time.Duration
	continuity bool // if true, keep triggering every interval
	prompt   string

	lastTrigger time.Time
	lastReset   time.Time // tracks last user activity reset

	// Time function for testing
	now func() time.Time
}

// New creates a new Muse instance from config.
func New(cfg *config.Config) *Muse {
	c := GetConfig(cfg)
	return &Muse{
		enabled:    c.Enabled,
		interval:    c.Interval,
		continuity: c.Continuity,
		prompt:     c.Prompt,
		now:        time.Now,
	}
}

// IsEnabled returns true if Muse is enabled.
func (m *Muse) IsEnabled() bool {
	return m.enabled
}

// Interval returns the configured interval.
func (m *Muse) Interval() time.Duration {
	return m.interval
}

// Prompt returns the configured prompt.
func (m *Muse) Prompt() string {
	return m.prompt
}

// Continuity returns true if continuous triggering is enabled.
func (m *Muse) Continuity() bool {
	return m.continuity
}

// Tick returns a command that sends TickMsg after the tick interval.
func (m *Muse) Tick() tea.Cmd {
	return tea.Tick(TickInterval, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// ShouldTrigger checks if Muse should be triggered.
// hasSession and isBusy are external states that must be provided.
func (m *Muse) ShouldTrigger(elapsed time.Duration, hasSession, isBusy bool) bool {
	if !m.enabled {
		return false
	}
	if !hasSession {
		return false
	}
	if isBusy {
		return false
	}

	// Check if user has been inactive long enough
	if elapsed < m.interval {
		return false
	}

	now := m.now()

	// First trigger: never triggered before
	if m.lastTrigger.IsZero() {
		// Check if enough time has passed since last reset
		if !m.lastReset.IsZero() && now.Sub(m.lastReset) < m.interval {
			return false
		}
		slog.Debug("Muse: first trigger after interval")
		return true
	}

	// Subsequent triggers: check continuity setting
	if m.continuity {
		// Keep triggering every interval
		timeSinceTrigger := now.Sub(m.lastTrigger)
		if timeSinceTrigger >= m.interval {
			slog.Debug("Muse: recurring trigger", "since_last", timeSinceTrigger)
			return true
		}
	}

	// Single mode: only trigger once per inactivity period
	slog.Debug("Muse: already triggered (single mode)")
	return false
}

// MarkTriggered records that a trigger has occurred.
// This MUST be called from Update() before dispatching the Trigger command.
func (m *Muse) MarkTriggered() {
	m.lastTrigger = m.now()
}

// Trigger returns a command that executes the Muse thinking.
// This is a pure command - it does NOT mutate any state.
// State mutations (MarkTriggered) should happen in Update() before calling this.
//
// The context should be created and managed by the caller (UI model),
// which allows proper cancellation handling.
func (m *Muse) Trigger(ctx context.Context, sessionID string, run RunFunc) tea.Cmd {
	prompt := m.prompt // Capture prompt value at command creation time
	return func() tea.Msg {
		err := run(ctx, sessionID, prompt)
		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("Muse: run failed", "error", err)
		}
		return TriggerCompleteMsg{Prompt: prompt, Error: err}
	}
}

// Reset resets the Muse trigger state on user activity.
// This should be called from Update() when user activity is detected.
func (m *Muse) Reset() {
	m.lastTrigger = time.Time{}
	m.lastReset = m.now()
}

// UpdateConfig updates Muse settings from config.
// Note: enabled is runtime state and is NOT updated from config.
func (m *Muse) UpdateConfig(cfg *config.Config) tea.Cmd {
	c := GetConfig(cfg)
	m.interval = c.Interval
	m.continuity = c.Continuity
	m.prompt = c.Prompt

	if m.enabled {
		return m.Tick()
	}
	return nil
}

// SetEnabled toggles the enabled state.
func (m *Muse) SetEnabled(enabled bool, cfg *config.Config) tea.Cmd {
	m.enabled = enabled
	if enabled {
		return m.Tick()
	}
	return nil
}

// SetInterval sets interval value.
func (m *Muse) SetInterval(interval int, cfg *config.Config) error {
	// Validate range: 1 second to 1 week
	const (
		minInterval = 1
		maxInterval = 604800 // 7 days in seconds
	)
	if interval < minInterval || interval > maxInterval {
		return fmt.Errorf("invalid interval value: must be between %d and %d (1 week)", minInterval, maxInterval)
	}
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	cfg.Options.MuseInterval = interval
	m.interval = time.Duration(interval) * time.Second
	if err := cfg.SetConfigField("options.muse_interval", interval); err != nil {
		return fmt.Errorf("failed to save Muse interval: %w", err)
	}
	return nil
}

// SetPrompt sets the prompt text.
func (m *Muse) SetPrompt(prompt string, cfg *config.Config) error {
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	cfg.Options.MusePrompt = prompt
	m.prompt = prompt
	if err := cfg.SetConfigField("options.muse_prompt", prompt); err != nil {
		return fmt.Errorf("failed to save Muse prompt: %w", err)
	}
	return nil
}

// SetContinuity sets the continuity value.
func (m *Muse) SetContinuity(continuity bool, cfg *config.Config) error {
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	cfg.Options.MuseContinuity = continuity
	m.continuity = continuity
	if err := cfg.SetConfigField("options.muse_continuity", continuity); err != nil {
		return fmt.Errorf("failed to save Muse continuity: %w", err)
	}
	return nil
}
