package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommandInput_BasicCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedCmd string
		expectedArgs []string
	}{
		{
			name:        "simple command no args",
			input:       `\review-pr`,
			expectedCmd: "review-pr",
			expectedArgs: []string{},
		},
		{
			name:        "command with single arg",
			input:       `\review-pr 123`,
			expectedCmd: "review-pr",
			expectedArgs: []string{"123"},
		},
		{
			name:        "command with multiple args",
			input:       `\review-pr 123 high priority`,
			expectedCmd: "review-pr",
			expectedArgs: []string{"123", "high", "priority"},
		},
		{
			name:        "namespaced command",
			input:       `\frontend:review-pr 123`,
			expectedCmd: "frontend:review-pr",
			expectedArgs: []string{"123"},
		},
		{
			name:        "nested namespaced command",
			input:       `\frontend:components:button test`,
			expectedCmd: "frontend:components:button",
			expectedArgs: []string{"test"},
		},
		{
			name:        "empty input",
			input:       "",
			expectedCmd: "",
			expectedArgs: nil,
		},
		{
			name:        "input without backslash",
			input:       "review-pr 123",
			expectedCmd: "",
			expectedArgs: nil,
		},
		{
			name:        "only backslash",
			input:       `\`,
			expectedCmd: "",
			expectedArgs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, args := ParseCommandInput(tt.input)
			assert.Equal(t, tt.expectedCmd, cmd)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestParseCommandInput_QuotedArguments(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedCmd string
		expectedArgs []string
	}{
		{
			name:        "double quoted arg",
			input:       `\review-pr 123 "high priority"`,
			expectedCmd: "review-pr",
			expectedArgs: []string{"123", "high priority"},
		},
		{
			name:        "single quoted arg",
			input:       `\review-pr 123 'high priority'`,
			expectedCmd: "review-pr",
			expectedArgs: []string{"123", "high priority"},
		},
		{
			name:        "multiple quoted args",
			input:       `\cmd "arg one" "arg two" arg3`,
			expectedCmd: "cmd",
			expectedArgs: []string{"arg one", "arg two", "arg3"},
		},
		{
			name:        "quoted arg with escaped quote",
			input:       `\cmd "arg \"with\" quotes"`,
			expectedCmd: "cmd",
			expectedArgs: []string{"arg \"with\" quotes"},
		},
		{
			name:        "mixed quoted and unquoted",
			input:       `\cmd arg1 "arg two" arg3 "arg four"`,
			expectedCmd: "cmd",
			expectedArgs: []string{"arg1", "arg two", "arg3", "arg four"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, args := ParseCommandInput(tt.input)
			assert.Equal(t, tt.expectedCmd, cmd)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestParseCommandInput_WhitespaceHandling(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedCmd string
		expectedArgs []string
	}{
		{
			name:        "leading whitespace",
			input:       `   \review-pr 123`,
			expectedCmd: "review-pr",
			expectedArgs: []string{"123"},
		},
		{
			name:        "trailing whitespace",
			input:       `\review-pr 123   `,
			expectedCmd: "review-pr",
			expectedArgs: []string{"123"},
		},
		{
			name:        "multiple spaces between args",
			input:       `\review-pr 123   456`,
			expectedCmd: "review-pr",
			expectedArgs: []string{"123", "456"},
		},
		{
			name:        "tabs and spaces",
			input:       "\\review-pr 123\t456",
			expectedCmd: "review-pr",
			expectedArgs: []string{"123", "456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, args := ParseCommandInput(tt.input)
			assert.Equal(t, tt.expectedCmd, cmd)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestParseArguments_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: []string{},
		},
		{
			name:     "single quoted empty string",
			input:    `""`,
			expected: []string{""},
		},
		{
			name:     "escaped backslash in quotes",
			input:    `"path\\to\\file"`,
			expected: []string{`path\to\file`},
		},
		{
			name:     "unclosed quote",
			input:    `"unclosed quote`,
			expected: []string{"unclosed quote"},
		},
		{
			name:     "single quoted empty",
			input:    `''`,
			expected: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseArguments(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

