package muse

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

func TestMuse_ShouldTrigger(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		enabled    bool
		hasSession bool
		isBusy     bool
		elapsed    time.Duration
		want       bool
	}{
		{
			name:       "disabled",
			enabled:    false,
			hasSession: true,
			isBusy:     false,
			elapsed:    15 * time.Second,
			want:       false,
		},
		{
			name:       "no session",
			enabled:    true,
			hasSession: false,
			isBusy:     false,
			elapsed:    15 * time.Second,
			want:       false,
		},
		{
			name:       "agent busy",
			enabled:    true,
			hasSession: true,
			isBusy:     true,
			elapsed:    15 * time.Second,
			want:       false,
		},
		{
			name:       "not enough elapsed time",
			enabled:    true,
			hasSession: true,
			isBusy:     false,
			elapsed:    5 * time.Second,
			want:       false,
		},
		{
			name:       "first trigger",
			enabled:    true,
			hasSession: true,
			isBusy:     false,
			elapsed:    15 * time.Second,
			want:       true,
		},
		{
			name:       "already triggered - single mode",
			enabled:    true,
			hasSession: true,
			isBusy:     false,
			elapsed:    15 * time.Second,
			want:       false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Muse{
				enabled:    tt.enabled,
				interval:   10 * time.Second,
				continuity: false,
				now:        time.Now,
			}
			if tt.name == "already triggered - single mode" {
				m.MarkTriggered()
			}
			got := m.ShouldTrigger(tt.elapsed, tt.hasSession, tt.isBusy)
			if got != tt.want {
				t.Errorf("ShouldTrigger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMuse_ShouldTrigger_ResetThenInterval(t *testing.T) {
	// Mock time function that returns a fixed time for testing
	now := time.Now()
	m := &Muse{
		enabled:  true,
		interval: 300 * time.Second,
		now:      func() time.Time { return now },
	}

	// First trigger: user inactive for 300s
	if !m.ShouldTrigger(300*time.Second, true, false) {
		t.Error("Should trigger after timeout")
	}

	// Mark as triggered
	m.MarkTriggered()

	// User presses key (reset) - simulate time has passed
	m.Reset()

	// Immediately after reset: should NOT trigger even if elapsed >= timeout
	if m.ShouldTrigger(300*time.Second, true, false) {
		t.Error("Should NOT trigger immediately after reset")
	}

	// Advance time by just before timeout (299s after reset)
	now = now.Add(299 * time.Second)
	if m.ShouldTrigger(300*time.Second, true, false) {
		t.Error("Should NOT trigger before timeout after reset")
	}

	// Advance time to exactly timeout (300s after reset)
	now = now.Add(1 * time.Second)
	if !m.ShouldTrigger(300*time.Second, true, false) {
		t.Error("Should trigger after timeout (from reset)")
	}
}

func TestMuse_Reset(t *testing.T) {
	t.Parallel()

	m := &Muse{
		enabled:     true,
		lastTrigger: time.Now(),
		now:         time.Now,
	}

	m.Reset()

	if !m.lastTrigger.IsZero() {
		t.Error("Reset should clear lastTrigger")
	}
	if m.lastReset.IsZero() {
		t.Error("Reset should set lastReset")
	}
}

func TestGetConfig(t *testing.T) {
	t.Parallel()

	t.Run("default values", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		c := GetConfig(cfg)

		if c.Enabled {
			t.Error("Should be disabled by default")
		}
		if c.Interval != DefaultInterval {
			t.Errorf("Interval = %v, want %v", c.Interval, DefaultInterval)
		}
		if c.Continuity {
			t.Error("Continuity should be false by default")
		}
		if c.Prompt != DefaultPrompt {
			t.Errorf("Prompt = %q, want %q", c.Prompt, DefaultPrompt)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Options: &config.Options{
				MuseInterval:   60,
				MuseContinuity: true,
				MusePrompt:     "Custom prompt",
			},
		}
		c := GetConfig(cfg)

		// Enabled is always false from config - it's a runtime state
		if c.Enabled {
			t.Error("Should be disabled by default")
		}
		if c.Interval != 60*time.Second {
			t.Errorf("Interval = %v, want %v", c.Interval, 60*time.Second)
		}
		if !c.Continuity {
			t.Error("Continuity should be true when set in config")
		}
		if c.Prompt != "Custom prompt" {
			t.Errorf("Prompt = %q, want %q", c.Prompt, "Custom prompt")
		}
	})
}

func TestMuse_PlaceholderText(t *testing.T) {
	t.Parallel()
	t.Run("shows countdown before first trigger", func(t *testing.T) {
		t.Parallel()

		m := &Muse{
			enabled:  true,
			interval: 120 * time.Second,
			now:      time.Now,
		}
		result := m.PlaceholderText(60*time.Second, false, true, false, "Ready")
		expected := "Muse in 1m"
		if result != expected {
			t.Errorf("PlaceholderText() = %q, want %q", result, expected)
		}
	})

	t.Run("shows Yolo and countdown", func(t *testing.T) {
		t.Parallel()

		m := &Muse{
			enabled:  true,
			interval: 120 * time.Second,
			now:      time.Now,
		}
		result := m.PlaceholderText(60*time.Second, true, true, false, "Ready")
		expected := "Yolo mode! Muse in 1m"
		if result != expected {
			t.Errorf("PlaceholderText() = %q, want %q", result, expected)
		}
	})

	t.Run("shows default when disabled", func(t *testing.T) {
		t.Parallel()

		m := &Muse{
			enabled:  false,
			interval: 120 * time.Second,
			now:      time.Now,
		}
		result := m.PlaceholderText(60*time.Second, false, true, false, "Ready")
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
	})

	t.Run("shows default when countdown reaches zero", func(t *testing.T) {
		t.Parallel()

		m := &Muse{
			enabled:  true,
			interval: 120 * time.Second,
			now:      time.Now,
		}
		result := m.PlaceholderText(130*time.Second, false, true, false, "Ready")
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
	})

	t.Run("shows default when no session", func(t *testing.T) {
		t.Parallel()

		m := &Muse{
			enabled:  true,
			interval: 120 * time.Second,
			now:      time.Now,
		}
		result := m.PlaceholderText(60*time.Second, false, false, false, "Ready")
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
	})

	t.Run("shows default when agent is busy", func(t *testing.T) {
		t.Parallel()

		m := &Muse{
			enabled:  true,
			interval: 120 * time.Second,
			now:      time.Now,
		}
		result := m.PlaceholderText(60*time.Second, false, true, true, "Ready")
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
	})
}

func TestMuse_New(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Options: &config.Options{
			MuseInterval:   60,
			MuseContinuity: true,
			MusePrompt:     "Test prompt",
		},
	}
	m := New(cfg)

	// Enabled is always false initially - it's a runtime state
	if m.enabled {
		t.Error("Should be disabled initially")
	}
	if m.interval != 60*time.Second {
		t.Errorf("Interval = %v, want %v", m.interval, 60*time.Second)
	}
	if !m.continuity {
		t.Error("Continuity should be true when set in config")
	}
	if m.prompt != "Test prompt" {
		t.Errorf("Prompt = %q, want %q", m.prompt, "Test prompt")
	}
	if m.now == nil {
		t.Error("now function should be set")
	}
}

func TestMuse_Tick(t *testing.T) {
	t.Parallel()

	m := &Muse{enabled: true}
	cmd := m.Tick()

	if cmd == nil {
		t.Error("Tick() should return a command")
	}

	// Execute the command and verify it returns TickMsg
	msg := cmd()
	if _, ok := msg.(TickMsg); !ok {
		t.Errorf("Tick() should return TickMsg, got %T", msg)
	}
}

func TestMuse_MarkTriggered(t *testing.T) {
	t.Parallel()

	m := &Muse{
		enabled:     true,
		lastTrigger: time.Time{},
		now:         time.Now,
	}

	m.MarkTriggered()

	if m.lastTrigger.IsZero() {
		t.Error("MarkTriggered should set lastTrigger")
	}
}

func TestMuse_Trigger(t *testing.T) {
	t.Parallel()

	m := &Muse{prompt: "Test prompt"}
	runCalled := false

	cmd := m.Trigger(context.Background(), "session-123", func(ctx context.Context, sessionID, prompt string) error {
		runCalled = true
		if sessionID != "session-123" {
			t.Errorf("sessionID = %q, want %q", sessionID, "session-123")
		}
		if prompt != "Test prompt" {
			t.Errorf("prompt = %q, want %q", prompt, "Test prompt")
		}
		return nil
	})

	if cmd == nil {
		t.Fatal("Trigger() should return a command")
	}

	msg := cmd()
	if !runCalled {
		t.Error("Run function should have been called")
	}

	completeMsg, ok := msg.(TriggerCompleteMsg)
	if !ok {
		t.Fatalf("Trigger() should return TriggerCompleteMsg, got %T", msg)
	}
	if completeMsg.Prompt != "Test prompt" {
		t.Errorf("Prompt = %q, want %q", completeMsg.Prompt, "Test prompt")
	}
	if completeMsg.Error != nil {
		t.Errorf("Error should be nil, got %v", completeMsg.Error)
	}
}

func TestMuse_Trigger_WithError(t *testing.T) {
	t.Parallel()

	m := &Muse{prompt: "Test"}

	cmd := m.Trigger(context.Background(), "session", func(ctx context.Context, sessionID, prompt string) error {
		return assertError("test error")
	})

	msg := cmd()
	completeMsg, ok := msg.(TriggerCompleteMsg)
	if !ok {
		t.Fatalf("Trigger() should return TriggerCompleteMsg, got %T", msg)
	}
	if completeMsg.Error == nil {
		t.Error("Error should not be nil")
	}
}

func assertError(msg string) error {
	return &testError{msg}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestMuse_UpdateConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Options: &config.Options{
			MuseInterval:   45,
			MuseContinuity: true,
			MusePrompt:     "Updated prompt",
		},
	}

	m := &Muse{
		enabled:    true, // runtime state - should NOT be changed by UpdateConfig
		interval:   120 * time.Second,
		continuity: false,
		prompt:     "Old prompt",
	}

	_ = m.UpdateConfig(cfg)

	// Enabled is runtime state, not affected by config
	if !m.enabled {
		t.Error("enabled should remain unchanged (runtime state)")
	}
	if m.interval != 45*time.Second {
		t.Errorf("interval = %v, want %v", m.interval, 45*time.Second)
	}
	if !m.continuity {
		t.Error("continuity should be updated from config")
	}
	if m.prompt != "Updated prompt" {
		t.Errorf("prompt = %q, want %q", m.prompt, "Updated prompt")
	}
	// cmd is nil because enabled is already true (no tick needed)
}

func TestMuse_SetEnabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		initial     bool
		setTo       bool
		wantEnabled bool
		wantCmd     bool
	}{
		{
			name:        "enable when disabled",
			initial:     false,
			setTo:       true,
			wantEnabled: true,
			wantCmd:     true,
		},
		{
			name:        "disable when enabled",
			initial:     true,
			setTo:       false,
			wantEnabled: false,
			wantCmd:     false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{Options: &config.Options{}}
			m := &Muse{enabled: tt.initial}
			cmd := m.SetEnabled(tt.setTo, cfg)

			if m.enabled != tt.wantEnabled {
				t.Errorf("enabled = %v, want %v", m.enabled, tt.wantEnabled)
			}
			if (cmd != nil) != tt.wantCmd {
				t.Errorf("SetEnabled(%v) return nil = %v, want %v",
					tt.setTo, cmd == nil, tt.wantCmd)
			}
		})
	}
}

func TestMuse_SetInterval(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{}}
	m := &Muse{interval: 120 * time.Second}

	// Note: config save may fail in tests (no file path), but we verify the fields are updated
	_ = m.SetInterval(60, cfg)

	if m.interval != 60*time.Second {
		t.Errorf("interval = %v, want %v", m.interval, 60*time.Second)
	}
	if cfg.Options.MuseInterval != 60 {
		t.Errorf("config.MuseInterval = %v, want %v", cfg.Options.MuseInterval, 60)
	}
}

func TestMuse_SetInterval_OutOfRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		value       int
		expectError bool
		checkValue  int // For valid values, verify config was updated
	}{
		{
			name:        "negative value",
			value:       -1,
			expectError: true,
		},
		{
			name:        "zero value",
			value:       0,
			expectError: true,
		},
		{
			name:        "very large value",
			value:       999999,
			expectError: true,
		},
		{
			name:        "extremely large value",
			value:       999999999,
			expectError: true,
		},
		{
			name:        "above maximum",
			value:       604801,
			expectError: true,
		},
		{
			name:        "minimum boundary",
			value:       1,
			expectError: false,
			checkValue:  1,
		},
		{
			name:        "maximum boundary",
			value:       604800,
			expectError: false,
			checkValue:  604800,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{Options: &config.Options{}}
			m := &Muse{interval: 120 * time.Second}
			err := m.SetInterval(tt.value, cfg)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for value %d, got nil", tt.value)
				}
			} else {
				// Note: config save may fail in tests (no file path), but we verify the fields are updated
				if tt.checkValue > 0 && cfg.Options.MuseInterval != tt.checkValue {
					t.Errorf("config.MuseInterval = %v, want %v",
						cfg.Options.MuseInterval, tt.checkValue)
				}
			}
		})
	}
}

func TestMuse_SetPrompt(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Options: &config.Options{}}
	m := &Muse{prompt: "Old"}

	// Note: config save may fail in tests (no file path), but we verify the fields are updated
	_ = m.SetPrompt("New prompt", cfg)

	if m.prompt != "New prompt" {
		t.Errorf("prompt = %q, want %q", m.prompt, "New prompt")
	}
	if cfg.Options.MusePrompt != "New prompt" {
		t.Errorf("config.MusePrompt = %q, want %q", cfg.Options.MusePrompt, "New prompt")
	}
}

func TestMuse_SetInterval_NilOptions(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Options: nil}
	m := &Muse{}

	m.SetInterval(60, cfg)

	if cfg.Options == nil {
		t.Error("Options should be created")
	}
}

func TestMuse_Continuity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		initial     bool
		expectedGet bool
	}{
		{
			name:        "returns true when continuity is true",
			initial:     true,
			expectedGet: true,
		},
		{
			name:        "returns false when continuity is false",
			initial:     false,
			expectedGet: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Muse{continuity: tt.initial}
			got := m.Continuity()
			if got != tt.expectedGet {
				t.Errorf("Continuity() = %v, want %v", got, tt.expectedGet)
			}
		})
	}
}

func TestMuse_SetContinuity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		initial bool
		setTo   bool
		want    bool
	}{
		{
			name:    "set to true",
			initial: false,
			setTo:   true,
			want:    true,
		},
		{
			name:    "set to false",
			initial: true,
			setTo:   false,
			want:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{Options: &config.Options{}}
			m := &Muse{continuity: tt.initial}
			_ = m.SetContinuity(tt.setTo, cfg)

			if m.continuity != tt.want {
				t.Errorf("continuity = %v, want %v", m.continuity, tt.want)
			}
			if cfg.Options.MuseContinuity != tt.want {
				t.Errorf("config.MuseContinuity = %v, want %v",
					cfg.Options.MuseContinuity, tt.want)
			}
		})
	}
}

func TestMuse_SetContinuity_NilOptions(t *testing.T) {
	cfg := &config.Config{Options: nil}
	m := &Muse{}

	m.SetContinuity(true, cfg)

	if cfg.Options == nil {
		t.Error("Options should be created")
	}
}

func TestMuse_ShouldTrigger_ContinuityMode(t *testing.T) {
	now := time.Now()
	m := &Muse{
		enabled:    true,
		interval:   10 * time.Second,
		continuity: true,
		now:        func() time.Time { return now },
	}

	// First trigger after timeout
	if !m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should trigger on first trigger after timeout")
	}

	// Mark as triggered
	m.MarkTriggered()
	now = now.Add(15 * time.Second) // Advance time

	// Second trigger should happen in continuity mode
	if !m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should trigger again in continuity mode")
	}

	// Mark as triggered again
	m.MarkTriggered()
	now = now.Add(15 * time.Second) // Advance time

	// Third trigger should also happen
	if !m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should trigger again in continuity mode (third time)")
	}
}

func TestMuse_ShouldTrigger_SingleMode(t *testing.T) {
	now := time.Now()
	m := &Muse{
		enabled:    true,
		interval:   10 * time.Second,
		continuity: false,
		now:        func() time.Time { return now },
	}

	// First trigger after timeout
	if !m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should trigger on first trigger after timeout")
	}

	// Mark as triggered
	m.MarkTriggered()
	now = now.Add(15 * time.Second) // Advance time

	// Second trigger should NOT happen in single mode
	if m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should NOT trigger again in single mode")
	}
}

func TestGetConfig_Continuity(t *testing.T) {
	t.Parallel()

	t.Run("default continuity is false", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		c := GetConfig(cfg)

		if c.Continuity {
			t.Error("Continuity should be false by default")
		}
	})

	t.Run("custom continuity value", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Options: &config.Options{
				MuseContinuity: true,
			},
		}
		c := GetConfig(cfg)

		if !c.Continuity {
			t.Error("Continuity should be true when set in config")
		}
	})

	t.Run("continuity in custom config", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Options: &config.Options{
				MuseInterval:   60,
				MuseContinuity: true,
				MusePrompt:     "Test",
			},
		}
		c := GetConfig(cfg)

		if !c.Continuity {
			t.Error("Continuity should be true")
		}
		if c.Interval != 60*time.Second {
			t.Errorf("Interval = %v, want %v", c.Interval, 60*time.Second)
		}
		if c.Prompt != "Test" {
			t.Errorf("Prompt = %q, want %q", c.Prompt, "Test")
		}
	})
}

func TestMuse_WillTrigger(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		enabled     bool
		continuity  bool
		triggered   bool
		wantTrigger bool
	}{
		{
			name:        "disabled",
			enabled:     false,
			continuity:  false,
			triggered:   false,
			wantTrigger: false,
		},
		{
			name:        "enabled not triggered",
			enabled:     true,
			continuity:  false,
			triggered:   false,
			wantTrigger: true,
		},
		{
			name:        "single mode already triggered",
			enabled:     true,
			continuity:  false,
			triggered:   true,
			wantTrigger: false,
		},
		{
			name:        "continuity mode not triggered",
			enabled:     true,
			continuity:  true,
			triggered:   false,
			wantTrigger: true,
		},
		{
			name:        "continuity mode already triggered",
			enabled:     true,
			continuity:  true,
			triggered:   true,
			wantTrigger: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &Muse{
				enabled:    tt.enabled,
				continuity: tt.continuity,
				now:        time.Now,
			}
			if tt.triggered {
				m.MarkTriggered()
			}

			got := m.WillTrigger()
			if got != tt.wantTrigger {
				t.Errorf("WillTrigger() = %v, want %v", got, tt.wantTrigger)
			}
		})
	}
}

func TestMuse_PlaceholderText_AlreadyTriggered(t *testing.T) {
	t.Parallel()

	t.Run("shows default when already triggered in single mode", func(t *testing.T) {
		t.Parallel()

		m := &Muse{
			enabled:    true,
			interval:   120 * time.Second,
			continuity: false,
			now:        time.Now,
		}
		m.MarkTriggered()

		result := m.PlaceholderText(60*time.Second, false, true, false, "Ready")
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q (should not show countdown after trigger)", result, "Ready")
		}
	})

	t.Run("shows countdown when already triggered in continuity mode", func(t *testing.T) {
		t.Parallel()

		m := &Muse{
			enabled:    true,
			interval:   120 * time.Second,
			continuity: true,
			now:        time.Now,
		}
		m.MarkTriggered()

		result := m.PlaceholderText(60*time.Second, false, true, false, "Ready")
		expected := "Muse in 1m"
		if result != expected {
			t.Errorf("PlaceholderText() = %q, want %q (should show countdown in continuity mode)", result, expected)
		}
	})
}
