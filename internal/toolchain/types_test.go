package toolchain

import (
	"testing"
	"time"
)

func TestToolCallDuration(t *testing.T) {
	tests := []struct {
		name     string
		call     ToolCall
		expected time.Duration
	}{
		{
			name: "valid duration",
			call: ToolCall{
				StartedAt:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				FinishedAt: time.Date(2024, 1, 1, 12, 0, 5, 0, time.UTC),
			},
			expected: 5 * time.Second,
		},
		{
			name: "zero started",
			call: ToolCall{
				FinishedAt: time.Date(2024, 1, 1, 12, 0, 5, 0, time.UTC),
			},
			expected: 0,
		},
		{
			name: "zero finished",
			call: ToolCall{
				StartedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			expected: 0,
		},
		{
			name:     "both zero",
			call:     ToolCall{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.call.Duration()
			if got != tt.expected {
				t.Errorf("Duration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChainAdd(t *testing.T) {
	chain := &Chain{
		SessionID: "session-1",
		MessageID: "msg-1",
	}

	call1 := ToolCall{
		ID:         "call-1",
		Name:       "bash",
		StartedAt:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2024, 1, 1, 12, 0, 5, 0, time.UTC),
	}
	call2 := ToolCall{
		ID:         "call-2",
		Name:       "view",
		StartedAt:  time.Date(2024, 1, 1, 12, 0, 5, 0, time.UTC),
		FinishedAt: time.Date(2024, 1, 1, 12, 0, 10, 0, time.UTC),
	}

	chain.Add(call1)
	if chain.Len() != 1 {
		t.Errorf("Len() = %d, want 1", chain.Len())
	}
	if chain.StartedAt != call1.StartedAt {
		t.Errorf("StartedAt = %v, want %v", chain.StartedAt, call1.StartedAt)
	}

	chain.Add(call2)
	if chain.Len() != 2 {
		t.Errorf("Len() = %d, want 2", chain.Len())
	}
	if chain.FinishedAt != call2.FinishedAt {
		t.Errorf("FinishedAt = %v, want %v", chain.FinishedAt, call2.FinishedAt)
	}
}

func TestChainDuration(t *testing.T) {
	chain := &Chain{
		StartedAt:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2024, 1, 1, 12, 1, 30, 0, time.UTC),
	}

	expected := 90 * time.Second
	if got := chain.Duration(); got != expected {
		t.Errorf("Duration() = %v, want %v", got, expected)
	}
}

func TestChainIsEmpty(t *testing.T) {
	empty := &Chain{}
	if !empty.IsEmpty() {
		t.Error("IsEmpty() = false, want true for empty chain")
	}

	nonEmpty := &Chain{
		Calls: []ToolCall{{ID: "call-1"}},
	}
	if nonEmpty.IsEmpty() {
		t.Error("IsEmpty() = true, want false for non-empty chain")
	}
}

func TestChainLast(t *testing.T) {
	empty := &Chain{}
	if got := empty.Last(); got != nil {
		t.Errorf("Last() = %v, want nil for empty chain", got)
	}

	chain := &Chain{
		Calls: []ToolCall{
			{ID: "call-1", Name: "bash"},
			{ID: "call-2", Name: "view"},
		},
	}
	last := chain.Last()
	if last == nil {
		t.Fatal("Last() = nil, want non-nil")
	}
	if last.ID != "call-2" {
		t.Errorf("Last().ID = %q, want %q", last.ID, "call-2")
	}
}

func TestChainToolNames(t *testing.T) {
	chain := &Chain{
		Calls: []ToolCall{
			{Name: "bash"},
			{Name: "view"},
			{Name: "bash"},
			{Name: "edit"},
			{Name: "view"},
		},
	}

	names := chain.ToolNames()
	expected := []string{"bash", "view", "edit"}

	if len(names) != len(expected) {
		t.Errorf("ToolNames() returned %d names, want %d", len(names), len(expected))
	}

	for i, name := range expected {
		if names[i] != name {
			t.Errorf("ToolNames()[%d] = %q, want %q", i, names[i], name)
		}
	}
}

func TestChainHasErrors(t *testing.T) {
	noErrors := &Chain{
		Calls: []ToolCall{
			{IsError: false},
			{IsError: false},
		},
	}
	if noErrors.HasErrors() {
		t.Error("HasErrors() = true, want false")
	}

	withErrors := &Chain{
		Calls: []ToolCall{
			{IsError: false},
			{IsError: true},
			{IsError: false},
		},
	}
	if !withErrors.HasErrors() {
		t.Error("HasErrors() = false, want true")
	}
}

func TestChainErrorCount(t *testing.T) {
	chain := &Chain{
		Calls: []ToolCall{
			{IsError: false},
			{IsError: true},
			{IsError: false},
			{IsError: true},
			{IsError: true},
		},
	}

	if got := chain.ErrorCount(); got != 3 {
		t.Errorf("ErrorCount() = %d, want 3", got)
	}
}

func TestNewSummary(t *testing.T) {
	chain := &Chain{
		SessionID: "session-1",
		Calls:     []ToolCall{{ID: "call-1"}},
	}

	summary := NewSummary(chain, "Ran 1 tool call")

	if summary.Chain != chain {
		t.Error("Summary.Chain does not match input chain")
	}
	if summary.Text != "Ran 1 tool call" {
		t.Errorf("Summary.Text = %q, want %q", summary.Text, "Ran 1 tool call")
	}
	if !summary.Collapsed {
		t.Error("Summary.Collapsed = false, want true")
	}
	if summary.GeneratedAt.IsZero() {
		t.Error("Summary.GeneratedAt is zero")
	}
}
