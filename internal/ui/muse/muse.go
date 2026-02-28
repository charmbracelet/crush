package muse

import (
	"context"
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
	enabled     bool
	timeout     time.Duration
	interval    time.Duration
	prompt      string
	lastTrigger time.Time
	wasBusy     bool // tracks previous agent state for interval reset

	// Time function for testing
	now func() time.Time
}

// New creates a new Muse instance from config.
func New(cfg *config.Config) *Muse {
	c := GetConfig(cfg)
	return &Muse{
		enabled:  c.Enabled,
		timeout:  c.Timeout,
		interval: c.Interval,
		prompt:   c.Prompt,
		now:      time.Now,
	}
}

// IsEnabled returns true if Muse is enabled.
func (m *Muse) IsEnabled() bool {
	return m.enabled
}

// Timeout returns the configured timeout.
func (m *Muse) Timeout() time.Duration {
	return m.timeout
}

// Interval returns the configured interval.
func (m *Muse) Interval() time.Duration {
	return m.interval
}

// Prompt returns the configured prompt.
func (m *Muse) Prompt() string {
	return m.prompt
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
	if elapsed < m.timeout {
		return false
	}

	// First trigger: never triggered before
	if m.lastTrigger.IsZero() {
		slog.Debug("Muse: first trigger")
		return true
	}

	// Subsequent triggers: check interval
	slog.Debug("Muse: checking interval", "interval", m.interval, "since_last", time.Since(m.lastTrigger))
	if m.interval <= 0 {
		// No interval set, only trigger once per inactivity period
		return false
	}
	return time.Since(m.lastTrigger) >= m.interval
}

// MarkTriggered records that a trigger has occurred.
// This MUST be called from Update() before dispatching the Trigger command.
func (m *Muse) MarkTriggered() {
	m.lastTrigger = m.now()
}

// AdjustLastTrigger shifts lastTrigger forward by the given duration.
// This is used to compensate for time spent while the agent was busy,
// preventing immediate re-triggering after agent finishes.
func (m *Muse) AdjustLastTrigger(d time.Duration) {
	if m.lastTrigger.IsZero() {
		return
	}
	m.lastTrigger = m.lastTrigger.Add(d)
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
		if err != nil {
			slog.Debug("Muse: run error", "error", err)
		}
		return TriggerCompleteMsg{Prompt: prompt, Error: err}
	}
}

// Reset resets the Muse trigger state on user activity.
// This should be called from Update() when user activity is detected.
func (m *Muse) Reset() {
	m.lastTrigger = time.Time{}
}

// MarkAgentFinished records that the agent just finished.
// This resets the interval timer so Muse won't trigger immediately
// after agent completes work.
// Returns true if the agent was busy and is now idle.
func (m *Muse) MarkAgentFinished() bool {
	if m.wasBusy {
		m.lastTrigger = m.now()
		m.wasBusy = false
		return true
	}
	return false
}

// UpdateAgentBusy tracks the agent busy state.
// Call this from Update() when agent busy state changes.
func (m *Muse) UpdateAgentBusy(busy bool) {
	m.wasBusy = busy
}

// UpdateConfig updates Muse settings from config.
func (m *Muse) UpdateConfig(cfg *config.Config) tea.Cmd {
	c := GetConfig(cfg)
	m.enabled = c.Enabled
	m.timeout = c.Timeout
	m.interval = c.Interval
	m.prompt = c.Prompt

	if m.enabled {
		return m.Tick()
	}
	return nil
}

// SetEnabled toggles the enabled state.
func (m *Muse) SetEnabled(enabled bool, cfg *config.Config) tea.Cmd {
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	cfg.Options.MuseEnabled = &enabled
	_ = cfg.SetConfigField("options.muse_enabled", enabled)

	m.enabled = enabled
	if enabled {
		return m.Tick()
	}
	return nil
}

// SetTimeout sets the timeout value.
func (m *Muse) SetTimeout(timeout int, cfg *config.Config) {
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	cfg.Options.MuseTimeout = timeout
	_ = cfg.SetConfigField("options.muse_timeout", timeout)
	m.timeout = time.Duration(timeout) * time.Second
}

// SetInterval sets the interval value.
func (m *Muse) SetInterval(interval int, cfg *config.Config) {
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	cfg.Options.MuseInterval = interval
	_ = cfg.SetConfigField("options.muse_interval", interval)
	m.interval = time.Duration(interval) * time.Second
	slog.Debug("Muse interval set", "value", interval)
}

// SetPrompt sets the prompt text.
func (m *Muse) SetPrompt(prompt string, cfg *config.Config) {
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	cfg.Options.MusePrompt = prompt
	_ = cfg.SetConfigField("options.muse_prompt", prompt)
	m.prompt = prompt
}
