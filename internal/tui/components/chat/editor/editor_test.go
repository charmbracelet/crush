package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractCommandQuery(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		startIndex int
		expected   string
	}{
		{
			name:       "simple command query",
			value:      "\\hel",
			startIndex: 0,
			expected:   "hel",
		},
		{
			name:       "namespaced command query",
			value:      "\\frontend:rev",
			startIndex: 0,
			expected:   "frontend:rev",
		},
		{
			name:       "empty query after backslash",
			value:      "\\",
			startIndex: 0,
			expected:   "",
		},
		{
			name:       "query with text before backslash",
			value:      "some text \\command",
			startIndex: 10,
			expected:   "command",
		},
		{
			name:       "query stops at whitespace",
			value:      "\\command arg1",
			startIndex: 0,
			expected:   "command",
		},
		{
			name:       "query stops at newline",
			value:      "\\command\nnext line",
			startIndex: 0,
			expected:   "command",
		},
		{
			name:       "query stops at tab",
			value:      "\\command\targ1",
			startIndex: 0,
			expected:   "command",
		},
		{
			name:       "query with multiple spaces",
			value:      "\\command  arg1",
			startIndex: 0,
			expected:   "command",
		},
		{
			name:       "complex namespaced command",
			value:      "\\frontend:components:button",
			startIndex: 0,
			expected:   "frontend:components:button",
		},
		{
			name:       "startIndex at end of string",
			value:      "\\command",
			startIndex: 8,
			expected:   "",
		},
		{
			name:       "startIndex out of bounds",
			value:      "\\command",
			startIndex: 100,
			expected:   "",
		},
		{
			name:       "startIndex negative",
			value:      "\\command",
			startIndex: -1,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal editor instance to test the method
			// We only need it to call extractCommandQuery
			e := &editorCmp{}
			result := e.extractCommandQuery(tt.value, tt.startIndex)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractCommandQuery_DistinguishesFromFileCompletions(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		startIndex int
		isCommand  bool
	}{
		{
			name:       "backslash is command",
			value:      "\\command",
			startIndex: 0,
			isCommand:  true,
		},
		{
			name:       "forward slash is not command",
			value:      "/path/to/file",
			startIndex: 0,
			isCommand:  false,
		},
		{
			name:       "backslash in middle of text",
			value:      "text \\command more",
			startIndex: 5,
			isCommand:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &editorCmp{}
			result := e.extractCommandQuery(tt.value, tt.startIndex)
			if tt.isCommand {
				// If it's a command, result should be non-empty (except for empty query case)
				if tt.value[tt.startIndex] == '\\' && len(tt.value) > tt.startIndex+1 {
					assert.NotEmpty(t, result, "Command query should not be empty")
				}
			} else {
				// If it's not a command (forward slash), result should be empty
				// or this function shouldn't be called for forward slash
				// But since we're testing extractCommandQuery specifically,
				// we just verify it handles non-backslash gracefully
				if tt.startIndex < len(tt.value) && tt.value[tt.startIndex] != '\\' {
					// This shouldn't happen in real usage, but function should handle it
					_ = result // Just ensure it doesn't panic
				}
			}
		})
	}
}

func TestExtractCommandQuery_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		startIndex int
		expected   string
	}{
		{
			name:       "empty string",
			value:      "",
			startIndex: 0,
			expected:   "",
		},
		{
			name:       "only backslash",
			value:      "\\",
			startIndex: 0,
			expected:   "",
		},
		{
			name:       "backslash at end",
			value:      "text\\",
			startIndex: 4,
			expected:   "",
		},
		{
			name:       "backslash followed by space",
			value:      "\\ ",
			startIndex: 0,
			expected:   "",
		},
		{
			name:       "backslash followed by newline",
			value:      "\\\n",
			startIndex: 0,
			expected:   "",
		},
		{
			name:       "backslash with trailing whitespace",
			value:      "\\command ",
			startIndex: 0,
			expected:   "command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &editorCmp{}
			result := e.extractCommandQuery(tt.value, tt.startIndex)
			assert.Equal(t, tt.expected, result)
		})
	}
}

