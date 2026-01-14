package messages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrettifyToolName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "known tool - agent",
			input:    "agent",
			expected: "Agent",
		},
		{
			name:     "known tool - bash",
			input:    "bash",
			expected: "Bash",
		},
		{
			name:     "known tool - edit",
			input:    "edit",
			expected: "Edit",
		},
		{
			name:     "known tool - view",
			input:    "view",
			expected: "View",
		},
		{
			name:     "unknown tool with underscores",
			input:    "mcp_serena_find_symbol",
			expected: "Mcp serena find symbol",
		},
		{
			name:     "unknown tool with hyphens",
			input:    "web-search-organic",
			expected: "Web search organic",
		},
		{
			name:     "unknown tool with mixed separators",
			input:    "my-custom_tool",
			expected: "My custom tool",
		},
		{
			name:     "already capitalized",
			input:    "CustomTool",
			expected: "CustomTool",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single character",
			input:    "x",
			expected: "X",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := prettifyToolName(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSubagentLikeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "single prompt param",
			input:    `{"prompt": "Search for files"}`,
			expected: true,
		},
		{
			name:     "prompt with other params",
			input:    `{"prompt": "Search", "path": "/src"}`,
			expected: false,
		},
		{
			name:     "no prompt param",
			input:    `{"query": "test"}`,
			expected: false,
		},
		{
			name:     "empty object",
			input:    `{}`,
			expected: false,
		},
		{
			name:     "invalid json",
			input:    `not json`,
			expected: false,
		},
		{
			name:     "prompt is not string",
			input:    `{"prompt": 123}`,
			expected: false,
		},
		{
			name:     "empty prompt string",
			input:    `{"prompt": ""}`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isSubagentLikeInput(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestPrettifyJSONParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single key extracts value",
			input:    `{"prompt": "Find the bug"}`,
			expected: "Find the bug",
		},
		{
			name:     "multiple keys as k=v",
			input:    `{"a": "1", "b": "2"}`,
			expected: "a=1, b=2",
		},
		{
			name:     "not json returns original",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "empty object",
			input:    `{}`,
			expected: `{}`,
		},
		{
			name:     "numeric value",
			input:    `{"count": 42}`,
			expected: "42",
		},
		{
			name:     "boolean value",
			input:    `{"enabled": true}`,
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := prettifyJSONParam(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestLooksLikeJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "object",
			input:    `{"key": "value"}`,
			expected: true,
		},
		{
			name:     "array",
			input:    `[1, 2, 3]`,
			expected: true,
		},
		{
			name:     "plain text",
			input:    "hello world",
			expected: false,
		},
		{
			name:     "starts with brace but not json",
			input:    "{not json",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "empty object",
			input:    `{}`,
			expected: true,
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := looksLikeJSON(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
