package toolchain

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("Enabled = false, want true")
	}
	if cfg.MinCalls != 3 {
		t.Errorf("MinCalls = %d, want 3", cfg.MinCalls)
	}
	if cfg.MaxOutputLength != 500 {
		t.Errorf("MaxOutputLength = %d, want 500", cfg.MaxOutputLength)
	}
	if !cfg.CollapseByDefault {
		t.Error("CollapseByDefault = false, want true")
	}
	if !cfg.IncludeTimings {
		t.Error("IncludeTimings = false, want true")
	}
	if !cfg.GroupByTool {
		t.Error("GroupByTool = false, want true")
	}
}

func TestConfigShouldSummarize(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		chain    *Chain
		expected bool
	}{
		{
			name:     "disabled config",
			cfg:      Config{Enabled: false, MinCalls: 1},
			chain:    &Chain{Calls: []ToolCall{{}, {}, {}}},
			expected: false,
		},
		{
			name:     "nil chain",
			cfg:      Config{Enabled: true, MinCalls: 1},
			chain:    nil,
			expected: false,
		},
		{
			name:     "empty chain",
			cfg:      Config{Enabled: true, MinCalls: 1},
			chain:    &Chain{},
			expected: false,
		},
		{
			name:     "below min calls",
			cfg:      Config{Enabled: true, MinCalls: 5},
			chain:    &Chain{Calls: []ToolCall{{}, {}, {}}},
			expected: false,
		},
		{
			name:     "at min calls",
			cfg:      Config{Enabled: true, MinCalls: 3},
			chain:    &Chain{Calls: []ToolCall{{}, {}, {}}},
			expected: true,
		},
		{
			name:     "above min calls",
			cfg:      Config{Enabled: true, MinCalls: 2},
			chain:    &Chain{Calls: []ToolCall{{}, {}, {}, {}, {}}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ShouldSummarize(tt.chain)
			if got != tt.expected {
				t.Errorf("ShouldSummarize() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name            string
		cfg             Config
		expectedMinCall int
		expectedMaxOut  int
	}{
		{
			name:            "valid config",
			cfg:             Config{MinCalls: 5, MaxOutputLength: 1000},
			expectedMinCall: 5,
			expectedMaxOut:  1000,
		},
		{
			name:            "negative min calls fixed",
			cfg:             Config{MinCalls: -1, MaxOutputLength: 500},
			expectedMinCall: 1,
			expectedMaxOut:  500,
		},
		{
			name:            "zero min calls fixed",
			cfg:             Config{MinCalls: 0, MaxOutputLength: 500},
			expectedMinCall: 1,
			expectedMaxOut:  500,
		},
		{
			name:            "negative max output fixed",
			cfg:             Config{MinCalls: 3, MaxOutputLength: -100},
			expectedMinCall: 3,
			expectedMaxOut:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err != nil {
				t.Fatalf("Validate() returned error: %v", err)
			}
			if tt.cfg.MinCalls != tt.expectedMinCall {
				t.Errorf("MinCalls = %d, want %d", tt.cfg.MinCalls, tt.expectedMinCall)
			}
			if tt.cfg.MaxOutputLength != tt.expectedMaxOut {
				t.Errorf("MaxOutputLength = %d, want %d", tt.cfg.MaxOutputLength, tt.expectedMaxOut)
			}
		})
	}
}

func TestConfigMerge(t *testing.T) {
	base := Config{
		Enabled:           true,
		MinCalls:          3,
		MaxOutputLength:   500,
		CollapseByDefault: true,
		IncludeTimings:    true,
		GroupByTool:       false,
	}

	other := Config{
		Enabled:           false,
		MinCalls:          5,
		MaxOutputLength:   1000,
		CollapseByDefault: false,
		IncludeTimings:    false,
		GroupByTool:       true,
	}

	result := base.Merge(other)

	if result.Enabled != false {
		t.Error("Enabled should be false from other")
	}
	if result.MinCalls != 5 {
		t.Errorf("MinCalls = %d, want 5", result.MinCalls)
	}
	if result.MaxOutputLength != 1000 {
		t.Errorf("MaxOutputLength = %d, want 1000", result.MaxOutputLength)
	}
	if result.CollapseByDefault != false {
		t.Error("CollapseByDefault should be false from other")
	}
	if result.IncludeTimings != false {
		t.Error("IncludeTimings should be false from other")
	}
	if result.GroupByTool != true {
		t.Error("GroupByTool should be true from other")
	}
}

func TestConfigMergeZeroValues(t *testing.T) {
	base := Config{
		Enabled:         true,
		MinCalls:        5,
		MaxOutputLength: 1000,
	}

	other := Config{
		Enabled:         true,
		MinCalls:        0,
		MaxOutputLength: 0,
	}

	result := base.Merge(other)

	// Zero values should not override
	if result.MinCalls != 5 {
		t.Errorf("MinCalls = %d, want 5 (zero should not override)", result.MinCalls)
	}
	if result.MaxOutputLength != 1000 {
		t.Errorf("MaxOutputLength = %d, want 1000 (zero should not override)", result.MaxOutputLength)
	}
}
