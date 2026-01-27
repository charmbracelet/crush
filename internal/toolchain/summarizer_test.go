package toolchain

import (
	"testing"
	"time"
)

func TestSummarizerConfigFromConfig(t *testing.T) {
	cfg := Config{
		MinCalls:       5,
		IncludeTimings: true,
	}

	summarizerCfg := SummarizerConfigFromConfig(cfg)

	if summarizerCfg.MinChainLength != 5 {
		t.Errorf("MinChainLength = %d, want 5", summarizerCfg.MinChainLength)
	}
	if summarizerCfg.IncludeDuration != true {
		t.Error("IncludeDuration should be true")
	}
	if summarizerCfg.CollapseThreshold != 5 {
		t.Errorf("CollapseThreshold = %d, want 5 (default)", summarizerCfg.CollapseThreshold)
	}
}

func TestNewSummarizerFromConfig(t *testing.T) {
	cfg := DefaultConfig()
	s := NewSummarizerFromConfig(cfg)

	if s == nil {
		t.Fatal("expected non-nil summarizer")
	}
	if s.config.MinChainLength != cfg.MinCalls {
		t.Errorf("MinChainLength = %d, want %d", s.config.MinChainLength, cfg.MinCalls)
	}
}

func TestSummarizer_Summarize(t *testing.T) {
	s := NewDefaultSummarizer()

	t.Run("empty chain returns nil", func(t *testing.T) {
		chain := &Chain{}
		summary := s.Summarize(chain)
		if summary != nil {
			t.Error("expected nil summary for empty chain")
		}
	})

	t.Run("nil chain returns nil", func(t *testing.T) {
		summary := s.Summarize(nil)
		if summary != nil {
			t.Error("expected nil summary for nil chain")
		}
	})

	t.Run("single call chain returns nil with default config", func(t *testing.T) {
		chain := &Chain{
			Calls: []ToolCall{
				{ID: "1", Name: "Bash", Output: "hello"},
			},
		}
		summary := s.Summarize(chain)
		if summary != nil {
			t.Error("expected nil summary for chain below min length")
		}
	})

	t.Run("two call chain returns summary", func(t *testing.T) {
		now := time.Now()
		chain := &Chain{
			Calls: []ToolCall{
				{ID: "1", Name: "Read", StartedAt: now, FinishedAt: now.Add(100 * time.Millisecond)},
				{ID: "2", Name: "Edit", StartedAt: now.Add(100 * time.Millisecond), FinishedAt: now.Add(200 * time.Millisecond)},
			},
			StartedAt:  now,
			FinishedAt: now.Add(200 * time.Millisecond),
		}
		summary := s.Summarize(chain)
		if summary == nil {
			t.Fatal("expected non-nil summary")
		}
		if summary.Text == "" {
			t.Error("expected non-empty summary text")
		}
		if summary.Chain != chain {
			t.Error("expected summary to reference original chain")
		}
	})

	t.Run("chain above collapse threshold is collapsed", func(t *testing.T) {
		chain := &Chain{}
		for i := 0; i < 6; i++ {
			chain.Calls = append(chain.Calls, ToolCall{
				ID:   string(rune('1' + i)),
				Name: "Bash",
			})
		}
		summary := s.Summarize(chain)
		if summary == nil {
			t.Fatal("expected non-nil summary")
		}
		if !summary.Collapsed {
			t.Error("expected chain to be collapsed")
		}
	})

	t.Run("chain below collapse threshold is not collapsed", func(t *testing.T) {
		chain := &Chain{
			Calls: []ToolCall{
				{ID: "1", Name: "Bash"},
				{ID: "2", Name: "Bash"},
			},
		}
		summary := s.Summarize(chain)
		if summary == nil {
			t.Fatal("expected non-nil summary")
		}
		if summary.Collapsed {
			t.Error("expected chain to not be collapsed")
		}
	})
}

func TestSummarizer_ShouldSummarize(t *testing.T) {
	s := NewDefaultSummarizer()

	tests := []struct {
		name     string
		chain    *Chain
		expected bool
	}{
		{"nil chain", nil, false},
		{"empty chain", &Chain{}, false},
		{"one call", &Chain{Calls: []ToolCall{{ID: "1"}}}, false},
		{"two calls", &Chain{Calls: []ToolCall{{ID: "1"}, {ID: "2"}}}, true},
		{"many calls", &Chain{Calls: []ToolCall{{ID: "1"}, {ID: "2"}, {ID: "3"}}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.ShouldSummarize(tt.chain)
			if result != tt.expected {
				t.Errorf("ShouldSummarize() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSummarizer_generateSummaryText(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		config   SummarizerConfig
		chain    *Chain
		contains []string
	}{
		{
			name:   "single tool type repeated",
			config: SummarizerConfig{MinChainLength: 1, IncludeDuration: false, IncludeErrors: false},
			chain: &Chain{
				Calls: []ToolCall{
					{Name: "Bash"},
					{Name: "Bash"},
					{Name: "Bash"},
				},
			},
			contains: []string{"3 tool calls", "Bash"},
		},
		{
			name:   "multiple tool types",
			config: SummarizerConfig{MinChainLength: 1, IncludeDuration: false, IncludeErrors: false, MaxToolsInSummary: 5},
			chain: &Chain{
				Calls: []ToolCall{
					{Name: "Read"},
					{Name: "Edit"},
					{Name: "Read"},
				},
			},
			contains: []string{"3 tool calls", "Read x2", "Edit"},
		},
		{
			name:   "with duration",
			config: SummarizerConfig{MinChainLength: 1, IncludeDuration: true, IncludeErrors: false},
			chain: &Chain{
				Calls:      []ToolCall{{Name: "Bash"}, {Name: "Bash"}},
				StartedAt:  now,
				FinishedAt: now.Add(2 * time.Second),
			},
			contains: []string{"2s"},
		},
		{
			name:   "with errors",
			config: SummarizerConfig{MinChainLength: 1, IncludeDuration: false, IncludeErrors: true},
			chain: &Chain{
				Calls: []ToolCall{
					{Name: "Bash", IsError: false},
					{Name: "Bash", IsError: true},
				},
			},
			contains: []string{"1 error"},
		},
		{
			name:   "multiple errors",
			config: SummarizerConfig{MinChainLength: 1, IncludeDuration: false, IncludeErrors: true},
			chain: &Chain{
				Calls: []ToolCall{
					{Name: "Bash", IsError: true},
					{Name: "Bash", IsError: true},
				},
			},
			contains: []string{"2 errors"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSummarizer(tt.config)
			text := s.generateSummaryText(tt.chain)
			for _, substr := range tt.contains {
				if !containsString(text, substr) {
					t.Errorf("summary %q does not contain %q", text, substr)
				}
			}
		})
	}
}

func TestSummarizer_PatternSummary(t *testing.T) {
	s := NewDefaultSummarizer()

	t.Run("file search pattern", func(t *testing.T) {
		chain := &Chain{
			Calls: []ToolCall{
				{Name: "Glob"},
				{Name: "Grep"},
				{Name: "Glob"},
			},
		}
		summary := s.PatternSummary(chain)
		if !containsString(summary, "Searched codebase") {
			t.Errorf("expected file search summary, got %q", summary)
		}
	})

	t.Run("bash sequence pattern", func(t *testing.T) {
		chain := &Chain{
			Calls: []ToolCall{
				{Name: "Bash"},
				{Name: "Bash"},
				{Name: "Bash"},
			},
		}
		summary := s.PatternSummary(chain)
		if !containsString(summary, "commands") {
			t.Errorf("expected bash sequence summary, got %q", summary)
		}
	})

	t.Run("read-modify-write pattern", func(t *testing.T) {
		chain := &Chain{
			Calls: []ToolCall{
				{Name: "Read"},
				{Name: "Edit"},
			},
		}
		summary := s.PatternSummary(chain)
		if !containsString(summary, "Read") && !containsString(summary, "modified") {
			t.Errorf("expected read-modify-write summary, got %q", summary)
		}
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Millisecond, "500ms"},
		{1 * time.Second, "1s"},
		{1500 * time.Millisecond, "1.5s"},
		{30 * time.Second, "30s"},
		{60 * time.Second, "1m"},
		{90 * time.Second, "1m30s"},
		{5 * time.Minute, "5m"},
		{5*time.Minute + 30*time.Second, "5m30s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestSummarizer_formatToolCounts(t *testing.T) {
	s := NewSummarizer(SummarizerConfig{MaxToolsInSummary: 3})

	t.Run("single tool single use", func(t *testing.T) {
		result := s.formatToolCounts(map[string]int{"Read": 1})
		if result != "Read" {
			t.Errorf("got %q, want %q", result, "Read")
		}
	})

	t.Run("single tool multiple uses", func(t *testing.T) {
		result := s.formatToolCounts(map[string]int{"Bash": 3})
		if result != "Bash x3" {
			t.Errorf("got %q, want %q", result, "Bash x3")
		}
	})

	t.Run("multiple tools sorted by frequency", func(t *testing.T) {
		result := s.formatToolCounts(map[string]int{"Read": 1, "Edit": 3, "Bash": 2})
		// Edit x3 should come first
		if !containsString(result, "Edit x3") {
			t.Errorf("expected Edit x3 in result %q", result)
		}
	})

	t.Run("exceeds max tools", func(t *testing.T) {
		result := s.formatToolCounts(map[string]int{
			"A": 5, "B": 4, "C": 3, "D": 2, "E": 1,
		})
		if !containsString(result, "+2 more") {
			t.Errorf("expected '+2 more' in result %q", result)
		}
	})
}

// containsString checks if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
