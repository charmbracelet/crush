package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
)

// mockTool is a simple mock implementation of fantasy.AgentTool for testing
type mockTool struct {
	name            string
	description     string
	providerOptions fantasy.ProviderOptions
}

func (m *mockTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{
		Name:        m.name,
		Description: m.description,
		Parameters:  map[string]any{},
	}
}

func (m *mockTool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.ToolResponse{Content: "mock result"}, nil
}

func (m *mockTool) ProviderOptions() fantasy.ProviderOptions {
	return m.providerOptions
}

func (m *mockTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	m.providerOptions = opts
}

func TestToClaudeCodeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"bash", "Bash"},
		{"read", "Read"},
		{"write", "Write"},
		{"edit", "Edit"},
		{"grep", "Grep"},
		{"glob", "Glob"},
		{"view", "View"},
		{"BASH", "Bash"},           // case insensitive
		{"custom", "custom"},       // not in the list
		{"multiedit", "multiedit"}, // not in the list
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToClaudeCodeName(tt.input)
			if result != tt.expected {
				t.Errorf("ToClaudeCodeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFromClaudeCodeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Bash", "bash"},
		{"Read", "read"},
		{"Write", "write"},
		{"Edit", "edit"},
		{"Grep", "grep"},
		{"Glob", "glob"},
		{"View", "view"},
		{"BASH", "bash"},     // case insensitive
		{"custom", "custom"}, // not in the list
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := FromClaudeCodeName(tt.input)
			if result != tt.expected {
				t.Errorf("FromClaudeCodeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestWrapToolsForOAuth(t *testing.T) {
	tools := []fantasy.AgentTool{
		&mockTool{name: "bash", description: "Run shell commands"},
		&mockTool{name: "read", description: "Read files"},
		&mockTool{name: "custom", description: "Custom tool"},
	}

	wrapped := WrapToolsForOAuth(tools)

	if len(wrapped) != 3 {
		t.Fatalf("WrapToolsForOAuth returned %d tools, want 3", len(wrapped))
	}

	// Check that bash was renamed to Bash
	if wrapped[0].Info().Name != "Bash" {
		t.Errorf("First tool name = %q, want %q", wrapped[0].Info().Name, "Bash")
	}

	// Check that read was renamed to Read
	if wrapped[1].Info().Name != "Read" {
		t.Errorf("Second tool name = %q, want %q", wrapped[1].Info().Name, "Read")
	}

	// Check that custom was NOT renamed
	if wrapped[2].Info().Name != "custom" {
		t.Errorf("Third tool name = %q, want %q", wrapped[2].Info().Name, "custom")
	}

	// Ensure description is preserved
	if wrapped[0].Info().Description != "Run shell commands" {
		t.Errorf("First tool description = %q, want %q", wrapped[0].Info().Description, "Run shell commands")
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that ToClaudeCodeName -> FromClaudeCodeName returns the original
	originals := []string{"bash", "read", "write", "edit", "grep", "glob", "view"}

	for _, orig := range originals {
		ccName := ToClaudeCodeName(orig)
		backToOrig := FromClaudeCodeName(ccName)
		if backToOrig != orig {
			t.Errorf("Round trip failed: %q -> %q -> %q", orig, ccName, backToOrig)
		}
	}
}
