package muse

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

func TestMuse_ShouldTrigger(t *testing.T) {
	// Create a Muse with mocked time (single mode by default)
	m := &Muse{
		enabled:    true,
		interval:   10 * time.Second,
		continuity: false,
		now:        time.Now,
	}

	// Test: not enabled
	m.enabled = false
	if m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should not trigger when disabled")
	}
	m.enabled = true

	// Test: no session
	if m.ShouldTrigger(15*time.Second, false, false) {
		t.Error("Should not trigger without session")
	}

	// Test: agent busy
	if m.ShouldTrigger(15*time.Second, true, true) {
		t.Error("Should not trigger when agent busy")
	}

	// Test: not enough elapsed time
	if m.ShouldTrigger(5*time.Second, true, false) {
		t.Error("Should not trigger before timeout")
	}

	// Test: first trigger
	if !m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should trigger on first trigger after timeout")
	}

	// Test: already triggered - should not trigger again (single mode)
	m.MarkTriggered()
	if m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should not trigger again in single mode")
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
	t.Run("default values", func(t *testing.T) {
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
	m := &Muse{
		enabled:  true,
		interval: 120 * time.Second,
		now:      time.Now,
	}

	t.Run("shows countdown before first trigger", func(t *testing.T) {
		m.lastTrigger = time.Time{}
		result := m.PlaceholderText(60*time.Second, false, true, false, "Ready")
		expected := "Muse in 1m"
		if result != expected {
			t.Errorf("PlaceholderText() = %q, want %q", result, expected)
		}
	})

	t.Run("shows Yolo and countdown", func(t *testing.T) {
		m.lastTrigger = time.Time{}
		result := m.PlaceholderText(60*time.Second, true, true, false, "Ready")
		expected := "Yolo mode! Muse in 1m"
		if result != expected {
			t.Errorf("PlaceholderText() = %q, want %q", result, expected)
		}
	})

	t.Run("shows default when disabled", func(t *testing.T) {
		m.enabled = false
		result := m.PlaceholderText(60*time.Second, false, true, false, "Ready")
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
		m.enabled = true
	})

	t.Run("shows default when countdown reaches zero", func(t *testing.T) {
		m.lastTrigger = time.Time{}
		result := m.PlaceholderText(130*time.Second, false, true, false, "Ready")
		// Countdown is negative, should show default
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
	})

	t.Run("shows default when no session", func(t *testing.T) {
		m.lastTrigger = time.Time{}
		// hasSession = false should hide countdown even when enabled
		result := m.PlaceholderText(60*time.Second, false, false, false, "Ready")
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
	})

	t.Run("shows default when agent is busy", func(t *testing.T) {
		m.lastTrigger = time.Time{}
		// isBusy = true should hide countdown even with session
		result := m.PlaceholderText(60*time.Second, false, true, true, "Ready")
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
	})
}

func TestMuse_New(t *testing.T) {
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
	cfg := &config.Config{Options: &config.Options{}}
	m := &Muse{enabled: false}

	cmd := m.SetEnabled(true, cfg)

	if !m.enabled {
		t.Error("enabled should be true")
	}
	if cmd == nil {
		t.Error("SetEnabled(true) should return tick command")
	}

	cmd = m.SetEnabled(false, cfg)
	if m.enabled {
		t.Error("enabled should be false")
	}
	if cmd != nil {
		t.Error("SetEnabled(false) should return nil")
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
	cfg := &config.Config{Options: &config.Options{}}
	m := &Muse{interval: 120 * time.Second}

	// Test negative values
	err := m.SetInterval(-1, cfg)
	if err == nil {
		t.Error("expected error for negative interval, got nil")
	}

	// Test zero
	err = m.SetInterval(0, cfg)
	if err == nil {
		t.Error("expected error for interval < 1, got nil")
	}

	// Test very large values (user case: 999999)
	err = m.SetInterval(999999, cfg)
	if err == nil {
		t.Error("expected error for interval > 604800, got nil")
	}

	// Test extremely large values (overflow case)
	err = m.SetInterval(999999999, cfg)
	if err == nil {
		t.Error("expected error for extremely large interval, got nil")
	}

	// Test maximum boundary
	err = m.SetInterval(604801, cfg)
	if err == nil {
		t.Error("expected error for interval > 604800, got nil")
	}

	// Test valid boundary values
	_ = m.SetInterval(1, cfg)
	if cfg.Options.MuseInterval != 1 {
		t.Errorf("config.MuseInterval = %v, want 1", cfg.Options.MuseInterval)
	}

	_ = m.SetInterval(604800, cfg)
	if cfg.Options.MuseInterval != 604800 {
		t.Errorf("config.MuseInterval = %v, want 604800", cfg.Options.MuseInterval)
	}
}

func TestMuse_SetPrompt(t *testing.T) {
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
	cfg := &config.Config{Options: nil}
	m := &Muse{}

	m.SetInterval(60, cfg)

	if cfg.Options == nil {
		t.Error("Options should be created")
	}
}

func TestMuse_SetPrompt_NilOptions(t *testing.T) {
	cfg := &config.Config{Options: nil}
	m := &Muse{}

	m.SetPrompt("Test", cfg)

	if cfg.Options == nil {
		t.Error("Options should be created")
	}
}

func TestMuse_Continuity(t *testing.T) {
	m := &Muse{continuity: true}

	if !m.Continuity() {
		t.Error("Continuity() should return true")
	}

	m.continuity = false
	if m.Continuity() {
		t.Error("Continuity() should return false")
	}
}

func TestMuse_SetContinuity(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{}}
	m := &Muse{continuity: false}

	// Test setting to true
	_ = m.SetContinuity(true, cfg)

	if !m.continuity {
		t.Error("continuity should be true")
	}
	if !cfg.Options.MuseContinuity {
		t.Error("config.MuseContinuity should be true")
	}

	// Test setting to false
	_ = m.SetContinuity(false, cfg)

	if m.continuity {
		t.Error("continuity should be false")
	}
	if cfg.Options.MuseContinuity {
		t.Error("config.MuseContinuity should be false")
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
	t.Run("default continuity is false", func(t *testing.T) {
		cfg := &config.Config{}
		c := GetConfig(cfg)

		if c.Continuity {
			t.Error("Continuity should be false by default")
		}
	})

	t.Run("custom continuity value", func(t *testing.T) {
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
