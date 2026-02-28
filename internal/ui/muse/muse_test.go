package muse

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

func TestFormatCountdown(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "0"},
		{1 * time.Second, "1"},
		{59 * time.Second, "59"},
		{60 * time.Second, "1:00"},
		{90 * time.Second, "1:30"},
		{120 * time.Second, "2:00"},
		{-1 * time.Second, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatCountdown(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatCountdown(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestMuse_ShouldTrigger(t *testing.T) {
	// Create a Muse with mocked time
	m := &Muse{
		enabled:  true,
		timeout:  10 * time.Second,
		interval: 5 * time.Second,
		now:      time.Now,
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

	// Test: already triggered, no interval
	m.lastTrigger = time.Now()
	m.interval = 0
	if m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should not trigger again with no interval")
	}

	// Test: already triggered, interval not elapsed
	m.interval = 5 * time.Second
	m.lastTrigger = time.Now()
	if m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should not trigger before interval elapsed")
	}

	// Test: already triggered, interval elapsed
	m.interval = 1 * time.Millisecond
	m.lastTrigger = time.Now().Add(-2 * time.Millisecond)
	if !m.ShouldTrigger(15*time.Second, true, false) {
		t.Error("Should trigger after interval elapsed")
	}
}

func TestMuse_Reset(t *testing.T) {
	m := &Muse{
		enabled:     true,
		lastTrigger: time.Now(),
	}

	m.Reset()

	if !m.lastTrigger.IsZero() {
		t.Error("Reset should clear lastTrigger")
	}
}

func TestMuse_UpdateAgentBusy(t *testing.T) {
	m := &Muse{
		enabled: true,
		wasBusy: false,
		now:     time.Now,
	}

	// Agent starts working
	m.UpdateAgentBusy(true)
	if !m.wasBusy {
		t.Error("wasBusy should be true after agent starts")
	}

	// Agent finishes - use MarkAgentFinished
	justFinished := m.MarkAgentFinished()
	if !justFinished {
		t.Error("Should report just finished when agent finishes")
	}
	if m.wasBusy {
		t.Error("wasBusy should be false after agent finishes")
	}
	if m.lastTrigger.IsZero() {
		t.Error("lastTrigger should be set when agent finishes")
	}
}

func TestGetConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		cfg := &config.Config{}
		c := GetConfig(cfg)

		if c.Enabled {
			t.Error("Should be disabled by default")
		}
		if c.Timeout != DefaultTimeout {
			t.Errorf("Timeout = %v, want %v", c.Timeout, DefaultTimeout)
		}
		if c.Interval != DefaultInterval {
			t.Errorf("Interval = %v, want %v", c.Interval, DefaultInterval)
		}
		if c.Prompt != DefaultPrompt {
			t.Errorf("Prompt = %q, want %q", c.Prompt, DefaultPrompt)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		enabled := true
		cfg := &config.Config{
			Options: &config.Options{
				MuseEnabled:  &enabled,
				MuseTimeout:  60,
				MuseInterval: 300,
				MusePrompt:   "Custom prompt",
			},
		}
		c := GetConfig(cfg)

		if !c.Enabled {
			t.Error("Should be enabled")
		}
		if c.Timeout != 60*time.Second {
			t.Errorf("Timeout = %v, want %v", c.Timeout, 60*time.Second)
		}
		if c.Interval != 300*time.Second {
			t.Errorf("Interval = %v, want %v", c.Interval, 300*time.Second)
		}
		if c.Prompt != "Custom prompt" {
			t.Errorf("Prompt = %q, want %q", c.Prompt, "Custom prompt")
		}
	})
}

func TestMuse_PlaceholderText(t *testing.T) {
	m := &Muse{
		enabled:  true,
		timeout:  120 * time.Second,
		interval: 60 * time.Second,
	}

	t.Run("shows countdown before first trigger", func(t *testing.T) {
		m.lastTrigger = time.Time{}
		result := m.PlaceholderText(60*time.Second, false, "Ready")
		expected := "Muse in 1:00"
		if result != expected {
			t.Errorf("PlaceholderText() = %q, want %q", result, expected)
		}
	})

	t.Run("shows Yolo and countdown", func(t *testing.T) {
		m.lastTrigger = time.Time{}
		result := m.PlaceholderText(60*time.Second, true, "Ready")
		expected := "Yolo mode! Muse in 1:00"
		if result != expected {
			t.Errorf("PlaceholderText() = %q, want %q", result, expected)
		}
	})

	t.Run("shows default when disabled", func(t *testing.T) {
		m.enabled = false
		result := m.PlaceholderText(60*time.Second, false, "Ready")
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
		m.enabled = true
	})

	t.Run("shows default when countdown reaches zero", func(t *testing.T) {
		m.lastTrigger = time.Time{}
		result := m.PlaceholderText(130*time.Second, false, "Ready")
		// Countdown is negative, should show default
		if result != "Ready" {
			t.Errorf("PlaceholderText() = %q, want %q", result, "Ready")
		}
	})
}

func TestMuse_New(t *testing.T) {
	enabled := true
	cfg := &config.Config{
		Options: &config.Options{
			MuseEnabled:  &enabled,
			MuseTimeout:  60,
			MuseInterval: 120,
			MusePrompt:   "Test prompt",
		},
	}
	m := New(cfg)

	if !m.enabled {
		t.Error("Should be enabled")
	}
	if m.timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", m.timeout, 60*time.Second)
	}
	if m.interval != 120*time.Second {
		t.Errorf("Interval = %v, want %v", m.interval, 120*time.Second)
	}
	if m.prompt != "Test prompt" {
		t.Errorf("Prompt = %q, want %q", m.prompt, "Test prompt")
	}
	if m.now == nil {
		t.Error("now function should be set")
	}
}

func TestMuse_Getters(t *testing.T) {
	m := &Muse{
		enabled:  true,
		timeout:  30 * time.Second,
		interval: 60 * time.Second,
		prompt:   "Test",
	}

	if !m.IsEnabled() {
		t.Error("IsEnabled() should return true")
	}

	if m.Timeout() != 30*time.Second {
		t.Errorf("Timeout() = %v, want %v", m.Timeout(), 30*time.Second)
	}

	if m.Interval() != 60*time.Second {
		t.Errorf("Interval() = %v, want %v", m.Interval(), 60*time.Second)
	}

	if m.Prompt() != "Test" {
		t.Errorf("Prompt() = %q, want %q", m.Prompt(), "Test")
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

func TestMuse_AdjustLastTrigger(t *testing.T) {
	m := &Muse{
		lastTrigger: time.Now().Add(-10 * time.Second),
	}

	originalTrigger := m.lastTrigger
	m.AdjustLastTrigger(5 * time.Second)

	// After adjustment, lastTrigger should be 5 seconds later
	expectedDiff := time.Since(originalTrigger) - time.Since(m.lastTrigger)
	if expectedDiff < 4*time.Second || expectedDiff > 6*time.Second {
		t.Errorf("AdjustLastTrigger should shift lastTrigger by the given duration")
	}
}

func TestMuse_AdjustLastTrigger_ZeroValue(t *testing.T) {
	m := &Muse{
		lastTrigger: time.Time{}, // zero value
	}

	// Should not panic when lastTrigger is zero
	m.AdjustLastTrigger(5 * time.Second)

	if !m.lastTrigger.IsZero() {
		t.Error("AdjustLastTrigger should do nothing when lastTrigger is zero")
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

func TestMuse_MarkAgentFinished(t *testing.T) {
	t.Run("was busy, now idle", func(t *testing.T) {
		m := &Muse{
			wasBusy:     true,
			lastTrigger: time.Time{},
			now:         time.Now,
		}

		result := m.MarkAgentFinished()

		if !result {
			t.Error("MarkAgentFinished should return true when transitioning from busy to idle")
		}
		if m.wasBusy {
			t.Error("wasBusy should be false")
		}
		if m.lastTrigger.IsZero() {
			t.Error("lastTrigger should be set")
		}
	})

	t.Run("was already idle", func(t *testing.T) {
		m := &Muse{
			wasBusy:     false,
			lastTrigger: time.Time{},
		}

		result := m.MarkAgentFinished()

		if result {
			t.Error("MarkAgentFinished should return false when already idle")
		}
	})
}

func TestMuse_UpdateConfig(t *testing.T) {
	enabled := true
	cfg := &config.Config{
		Options: &config.Options{
			MuseEnabled:  &enabled,
			MuseTimeout:  45,
			MuseInterval: 90,
			MusePrompt:   "Updated prompt",
		},
	}

	m := &Muse{
		enabled:  false,
		timeout:  120 * time.Second,
		interval: 0,
		prompt:   "Old prompt",
	}

	cmd := m.UpdateConfig(cfg)

	if !m.enabled {
		t.Error("enabled should be updated")
	}
	if m.timeout != 45*time.Second {
		t.Errorf("timeout = %v, want %v", m.timeout, 45*time.Second)
	}
	if m.interval != 90*time.Second {
		t.Errorf("interval = %v, want %v", m.interval, 90*time.Second)
	}
	if m.prompt != "Updated prompt" {
		t.Errorf("prompt = %q, want %q", m.prompt, "Updated prompt")
	}
	if cmd == nil {
		t.Error("UpdateConfig should return tick command when enabled")
	}
}

func TestMuse_UpdateConfig_Disabled(t *testing.T) {
	disabled := false
	cfg := &config.Config{
		Options: &config.Options{
			MuseEnabled: &disabled,
		},
	}

	m := &Muse{enabled: true}
	cmd := m.UpdateConfig(cfg)

	if m.enabled {
		t.Error("enabled should be false")
	}
	if cmd != nil {
		t.Error("UpdateConfig should return nil when disabled")
	}
}

func TestMuse_SetEnabled(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{}}
	m := &Muse{enabled: false}

	cmd := m.SetEnabled(true, cfg)

	if !m.enabled {
		t.Error("enabled should be true")
	}
	if cfg.Options.MuseEnabled == nil || !*cfg.Options.MuseEnabled {
		t.Error("config should be updated")
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

func TestMuse_SetTimeout(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{}}
	m := &Muse{timeout: 120 * time.Second}

	m.SetTimeout(60, cfg)

	if m.timeout != 60*time.Second {
		t.Errorf("timeout = %v, want %v", m.timeout, 60*time.Second)
	}
	if cfg.Options.MuseTimeout != 60 {
		t.Errorf("config.MuseTimeout = %v, want %v", cfg.Options.MuseTimeout, 60)
	}
}

func TestMuse_SetInterval(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{}}
	m := &Muse{interval: 0}

	m.SetInterval(300, cfg)

	if m.interval != 300*time.Second {
		t.Errorf("interval = %v, want %v", m.interval, 300*time.Second)
	}
	if cfg.Options.MuseInterval != 300 {
		t.Errorf("config.MuseInterval = %v, want %v", cfg.Options.MuseInterval, 300)
	}
}

func TestMuse_SetPrompt(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{}}
	m := &Muse{prompt: "Old"}

	m.SetPrompt("New prompt", cfg)

	if m.prompt != "New prompt" {
		t.Errorf("prompt = %q, want %q", m.prompt, "New prompt")
	}
	if cfg.Options.MusePrompt != "New prompt" {
		t.Errorf("config.MusePrompt = %q, want %q", cfg.Options.MusePrompt, "New prompt")
	}
}

func TestMuse_SetEnabled_NilOptions(t *testing.T) {
	cfg := &config.Config{Options: nil}
	m := &Muse{enabled: false}

	m.SetEnabled(true, cfg)

	if !m.enabled {
		t.Error("enabled should be true")
	}
	if cfg.Options == nil {
		t.Error("Options should be created")
	}
}

func TestMuse_SetTimeout_NilOptions(t *testing.T) {
	cfg := &config.Config{Options: nil}
	m := &Muse{}

	m.SetTimeout(60, cfg)

	if cfg.Options == nil {
		t.Error("Options should be created")
	}
}

func TestMuse_SetInterval_NilOptions(t *testing.T) {
	cfg := &config.Config{Options: nil}
	m := &Muse{}

	m.SetInterval(300, cfg)

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
