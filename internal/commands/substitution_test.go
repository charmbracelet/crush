package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubstituteArguments_AllArguments(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		args     []string
		expected string
	}{
		{
			name:     "simple $ARGS",
			content:  "Review PR $ARGS",
			args:     []string{"123"},
			expected: "Review PR 123",
		},
		{
			name:     "simple $ARGUMENTS",
			content:  "Review PR $ARGUMENTS",
			args:     []string{"123"},
			expected: "Review PR 123",
		},
		{
			name:     "multiple args with $ARGS",
			content:  "Review PR $ARGS",
			args:     []string{"123", "high", "priority"},
			expected: "Review PR 123 high priority",
		},
		{
			name:     "multiple args with $ARGUMENTS",
			content:  "Review PR $ARGUMENTS",
			args:     []string{"123", "high", "priority"},
			expected: "Review PR 123 high priority",
		},
		{
			name:     "$ARGS in middle",
			content:  "Process: $ARGS and continue",
			args:     []string{"file1", "file2"},
			expected: "Process: file1 file2 and continue",
		},
		{
			name:     "$ARGUMENTS in middle",
			content:  "Process: $ARGUMENTS and continue",
			args:     []string{"file1", "file2"},
			expected: "Process: file1 file2 and continue",
		},
		{
			name:     "multiple $ARGS",
			content:  "First: $ARGS, Second: $ARGS",
			args:     []string{"test"},
			expected: "First: test, Second: test",
		},
		{
			name:     "multiple $ARGUMENTS",
			content:  "First: $ARGUMENTS, Second: $ARGUMENTS",
			args:     []string{"test"},
			expected: "First: test, Second: test",
		},
		{
			name:     "mixed $ARGS and $ARGUMENTS",
			content:  "First: $ARGS, Second: $ARGUMENTS",
			args:     []string{"test"},
			expected: "First: test, Second: test",
		},
		{
			name:     "empty args",
			content:  "Process $ARGUMENTS",
			args:     []string{},
			expected: "Process ",
		},
		{
			name:     "empty string arg",
			content:  "Process $ARGUMENTS",
			args:     []string{""},
			expected: "Process ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteArguments(tt.content, tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubstituteArguments_PositionalArguments(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		args     []string
		expected string
	}{
		{
			name:     "single $1",
			content:  "Review PR $1",
			args:     []string{"123"},
			expected: "Review PR 123",
		},
		{
			name:     "multiple sequential",
			content:  "Review PR $1 with priority $2",
			args:     []string{"123", "high"},
			expected: "Review PR 123 with priority high",
		},
		{
			name:     "non-sequential",
			content:  "Use $1 and $3",
			args:     []string{"a", "b", "c"},
			expected: "Use a and c",
		},
		{
			name:     "repeated positions",
			content:  "Use $1 multiple times: $1 again",
			args:     []string{"test"},
			expected: "Use test multiple times: test again",
		},
		{
			name:     "missing argument",
			content:  "Use $1 and $3",
			args:     []string{"a", "b"},
			expected: "Use a and ",
		},
		{
			name:     "large position number",
			content:  "Use $5",
			args:     []string{"a", "b", "c", "d", "e"},
			expected: "Use e",
		},
		{
			name:     "position beyond args",
			content:  "Use $10",
			args:     []string{"a", "b"},
			expected: "Use ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteArguments(tt.content, tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubstituteArguments_MixedPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		args     []string
		expected string
	}{
		{
			name:     "$ARGS and positional",
			content:  "Review $ARGS with priority $2",
			args:     []string{"123", "high"},
			expected: "Review 123 high with priority high",
		},
		{
			name:     "$ARGUMENTS and positional",
			content:  "Review $ARGUMENTS with priority $2",
			args:     []string{"123", "high"},
			expected: "Review 123 high with priority high",
		},
		{
			name:     "positional and $ARGS",
			content:  "PR $1: $ARGS",
			args:     []string{"123", "review", "needed"},
			expected: "PR 123: 123 review needed",
		},
		{
			name:     "positional and $ARGUMENTS",
			content:  "PR $1: $ARGUMENTS",
			args:     []string{"123", "review", "needed"},
			expected: "PR 123: 123 review needed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteArguments(tt.content, tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubstituteArguments_NoPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		args     []string
		expected string
	}{
		{
			name:     "no placeholders",
			content:  "Simple command text",
			args:     []string{"123"},
			expected: "Simple command text",
		},
		{
			name:     "dollar sign with number (treated as positional)",
			content:  "Price is $100",
			args:     []string{"test"},
			expected: "Price is ", // $100 matches as $100 positional arg, but args[99] doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteArguments(tt.content, tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubstituteArguments_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		args     []string
		expected string
	}{
		{
			name:     "empty content",
			content:  "",
			args:     []string{"test"},
			expected: "",
		},
		{
			name:     "empty args with $ARGS and positional",
			content:  "Process $ARGS and $1",
			args:     []string{},
			expected: "Process  and ",
		},
		{
			name:     "empty args with $ARGUMENTS and positional",
			content:  "Process $ARGUMENTS and $1",
			args:     []string{},
			expected: "Process  and ",
		},
		{
			name:     "args with spaces using $ARGS",
			content:  "Process $ARGS",
			args:     []string{"arg with spaces", "another"},
			expected: "Process arg with spaces another",
		},
		{
			name:     "args with spaces using $ARGUMENTS",
			content:  "Process $ARGUMENTS",
			args:     []string{"arg with spaces", "another"},
			expected: "Process arg with spaces another",
		},
		{
			name:     "zero position",
			content:  "Use $0",
			args:     []string{"test"},
			expected: "Use ", // $0 is invalid (1-based indexing)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteArguments(tt.content, tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

