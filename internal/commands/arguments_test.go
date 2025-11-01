package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractRequiredArguments_NoArguments(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "empty content",
			content: "",
		},
		{
			name:    "no placeholders",
			content: "This is a simple command with no arguments.",
		},
		{
			name:    "dollar sign followed by non-placeholder text",
			content: "Price is $USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRequiredArguments(tt.content, "")
			assert.False(t, result.HasAllArguments)
			assert.Equal(t, 0, result.MaxPositional)
			assert.Equal(t, 0, result.RequiredCount)
		})
	}
}

func TestExtractRequiredArguments_AllArguments(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "simple $ARGS",
			content: "Review PR $ARGS",
		},
		{
			name:    "simple $ARGUMENTS",
			content: "Review PR $ARGUMENTS",
		},
		{
			name:    "$ARGS in middle",
			content: "Process the following: $ARGS and continue",
		},
		{
			name:    "$ARGUMENTS in middle",
			content: "Process the following: $ARGUMENTS and continue",
		},
		{
			name:    "multiple $ARGS",
			content: "First: $ARGS, Second: $ARGS",
		},
		{
			name:    "multiple $ARGUMENTS",
			content: "First: $ARGUMENTS, Second: $ARGUMENTS",
		},
		{
			name:    "$ARGS with positional",
			content: "Use $ARGS and also $1",
		},
		{
			name:    "$ARGUMENTS with positional",
			content: "Use $ARGUMENTS and also $1",
		},
		{
			name:    "mixed $ARGS and $ARGUMENTS",
			content: "Use $ARGS and $ARGUMENTS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRequiredArguments(tt.content, "")
			assert.True(t, result.HasAllArguments)
			assert.Equal(t, -1, result.RequiredCount)
		})
	}
}

func TestExtractRequiredArguments_PositionalArguments(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedMax  int
		expectedCount int
	}{
		{
			name:         "single $1",
			content:      "Review PR $1",
			expectedMax:  1,
			expectedCount: 1,
		},
		{
			name:         "multiple sequential",
			content:      "Review PR $1 with priority $2",
			expectedMax:  2,
			expectedCount: 2,
		},
		{
			name:         "non-sequential",
			content:      "Use $1 and $3",
			expectedMax:  3,
			expectedCount: 3,
		},
		{
			name:         "large number",
			content:      "Process $1, $2, $3, $4, $5",
			expectedMax:  5,
			expectedCount: 5,
		},
		{
			name:         "repeated positions",
			content:      "Use $1 multiple times: $1 again",
			expectedMax:  1,
			expectedCount: 1,
		},
		{
			name:         "mixed with text",
			content:      "Review PR number $1 with priority $2 and assign to $3",
			expectedMax:  3,
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRequiredArguments(tt.content, "")
			assert.False(t, result.HasAllArguments)
			assert.Equal(t, tt.expectedMax, result.MaxPositional)
			assert.Equal(t, tt.expectedCount, result.RequiredCount)
		})
	}
}

func TestExtractRequiredArguments_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedMax  int
		expectedCount int
		hasAllArgs   bool
	}{
		{
			name:         "dollar followed by non-digit with $ARGS",
			content:      "Price is $100 but use $ARGS",
			hasAllArgs:   true,
			expectedCount: -1,
		},
		{
			name:         "dollar followed by non-digit with $ARGUMENTS",
			content:      "Price is $100 but use $ARGUMENTS",
			hasAllArgs:   true,
			expectedCount: -1,
		},
		{
			name:         "dollar at end of line",
			content:      "Use argument $1",
			expectedMax:  1,
			expectedCount: 1,
		},
		{
			name:         "dollar in quoted string",
			content:      "Process \"argument $1\"",
			expectedMax:  1,
			expectedCount: 1,
		},
		{
			name:         "very large number",
			content:      "Argument $999",
			expectedMax:  999,
			expectedCount: 999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRequiredArguments(tt.content, "")
			assert.Equal(t, tt.hasAllArgs, result.HasAllArguments)
			if !tt.hasAllArgs {
				assert.Equal(t, tt.expectedMax, result.MaxPositional)
			}
			assert.Equal(t, tt.expectedCount, result.RequiredCount)
		})
	}
}

func TestExtractRequiredArguments_ArgumentHint(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		argumentHint string
		expectedCount int
	}{
		{
			name:         "argument-hint with single argument",
			content:      "Sum two numbers provided by the user.",
			argumentHint: "[number1]",
			expectedCount: 1,
		},
		{
			name:         "argument-hint with two arguments",
			content:      "Sum two numbers provided by the user.",
			argumentHint: "[number1] [number2]",
			expectedCount: 2,
		},
		{
			name:         "argument-hint with three arguments",
			content:      "Process three values.",
			argumentHint: "[arg1] [arg2] [arg3]",
			expectedCount: 3,
		},
		{
			name:         "argument-hint can require more than content",
			content:      "Review PR $1",
			argumentHint: "[number1] [number2]",
			expectedCount: 2, // Argument-hint requires 2, content only has $1, so we require 2
		},
		{
			name:         "content with $ARGS takes precedence",
			content:      "Process $ARGS",
			argumentHint: "[number1] [number2]",
			expectedCount: -1, // $ARGS takes precedence
		},
		{
			name:         "content with $ARGUMENTS takes precedence",
			content:      "Process $ARGUMENTS",
			argumentHint: "[number1] [number2]",
			expectedCount: -1, // $ARGUMENTS takes precedence
		},
		{
			name:         "empty argument-hint",
			content:      "Simple command",
			argumentHint: "",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRequiredArguments(tt.content, tt.argumentHint)
			if tt.expectedCount == -1 {
				assert.True(t, result.HasAllArguments)
			} else {
				assert.Equal(t, tt.expectedCount, result.RequiredCount)
			}
		})
	}
}

func TestCountArgumentsFromHint(t *testing.T) {
	tests := []struct {
		name     string
		hint     string
		expected int
	}{
		{
			name:     "single argument",
			hint:     "[number1]",
			expected: 1,
		},
		{
			name:     "two arguments",
			hint:     "[number1] [number2]",
			expected: 2,
		},
		{
			name:     "three arguments",
			hint:     "[arg1] [arg2] [arg3]",
			expected: 3,
		},
		{
			name:     "empty string",
			hint:     "",
			expected: 0,
		},
		{
			name:     "no brackets",
			hint:     "number1 number2",
			expected: 0,
		},
		{
			name:     "nested brackets (edge case)",
			hint:     "[arg1 [nested]] [arg2]",
			expected: 2, // Regex matches [arg1 [nested]] and [arg2]
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countArgumentsFromHint(tt.hint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasArgumentPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "has $ARGS",
			content:  "Process $ARGS",
			expected: true,
		},
		{
			name:     "has $ARGUMENTS",
			content:  "Process $ARGUMENTS",
			expected: true,
		},
		{
			name:     "has $1",
			content:  "Review PR $1",
			expected: true,
		},
		{
			name:     "has $2",
			content:  "Use $1 and $2",
			expected: true,
		},
		{
			name:     "no placeholders",
			content:  "Sum two numbers provided by the user.",
			expected: false,
		},
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "dollar sign not placeholder",
			content:  "Price is $100",
			expected: true, // $100 matches $(\d+) pattern
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasArgumentPlaceholders(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasAllRequiredArguments(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		requiredCount int
		expected      bool
	}{
		{
			name:          "has $ARGS covers all",
			content:       "Process $ARGS",
			requiredCount: 5,
			expected:      true,
		},
		{
			name:          "has $ARGUMENTS covers all",
			content:       "Process $ARGUMENTS",
			requiredCount: 5,
			expected:      true,
		},
		{
			name:          "has all positional arguments",
			content:       "Use $1 and $2",
			requiredCount: 2,
			expected:      true,
		},
		{
			name:          "missing $2",
			content:       "Use $1",
			requiredCount: 2,
			expected:      false,
		},
		{
			name:          "missing $1",
			content:       "Use $2",
			requiredCount: 2,
			expected:      false,
		},
		{
			name:          "has $1 and $3 but missing $2",
			content:       "Use $1 and $3",
			requiredCount: 3,
			expected:      false,
		},
		{
			name:          "has all three arguments",
			content:       "Use $1, $2, and $3",
			requiredCount: 3,
			expected:      true,
		},
		{
			name:          "no arguments required",
			content:       "Simple command",
			requiredCount: 0,
			expected:      true,
		},
		{
			name:          "negative required count (means $ARGS)",
			content:       "Process $ARGS",
			requiredCount: -1,
			expected:      true,
		},
		{
			name:          "negative required count (means $ARGUMENTS)",
			content:       "Process $ARGUMENTS",
			requiredCount: -1,
			expected:      true,
		},
		{
			name:          "has extra arguments beyond required",
			content:       "Use $1, $2, $3, and $4",
			requiredCount: 2,
			expected:      true, // Has $1 and $2, which are all required
		},
		{
			name:          "repeated arguments count once",
			content:       "Use $1 multiple times: $1 again",
			requiredCount: 2,
			expected:      false, // Only has $1, missing $2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAllRequiredArguments(tt.content, tt.requiredCount)
			assert.Equal(t, tt.expected, result, "content: %q, requiredCount: %d", tt.content, tt.requiredCount)
		})
	}
}

