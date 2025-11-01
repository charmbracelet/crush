package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFileReferences_NoReferences(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "empty content",
			content:  "",
			expected: []string{},
		},
		{
			name:     "no references",
			content:  "This is a simple command with no file references.",
			expected: []string{},
		},
		{
			name:     "just at sign",
			content:  "Reference @ but no filename",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFileReferences(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseFileReferences_SingleReference(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "simple filename",
			content:  "Review @file.txt",
			expected: []string{"file.txt"},
		},
		{
			name:     "filename with path",
			content:  "Review @src/main.go",
			expected: []string{"src/main.go"},
		},
		{
			name:     "deep path",
			content:  "Process @path/to/deep/file.txt",
			expected: []string{"path/to/deep/file.txt"},
		},
		{
			name:     "relative path",
			content:  "Include @../parent/file.txt",
			expected: []string{"../parent/file.txt"},
		},
		{
			name:     "filename with dash",
			content:  "Use @my-file.txt",
			expected: []string{"my-file.txt"},
		},
		{
			name:     "filename with underscore",
			content:  "Load @my_file.txt",
			expected: []string{"my_file.txt"},
		},
		{
			name:     "reference in middle",
			content:  "Process @file.txt and continue",
			expected: []string{"file.txt"},
		},
		{
			name:     "reference at start",
			content:  "@file.txt is important",
			expected: []string{"file.txt"},
		},
		{
			name:     "reference at end",
			content:  "Review @file.txt",
			expected: []string{"file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFileReferences(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseFileReferences_MultipleReferences(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "two references",
			content:  "Review @file1.txt and @file2.go",
			expected: []string{"file1.txt", "file2.go"},
		},
		{
			name:     "three references",
			content:  "Process @file1 @file2 @file3",
			expected: []string{"file1", "file2", "file3"},
		},
		{
			name:     "references with paths",
			content:  "Review @src/main.go and @test/main_test.go",
			expected: []string{"src/main.go", "test/main_test.go"},
		},
		{
			name:     "scattered references",
			content:  "First @file1.txt, then @file2.go, finally @file3.md",
			expected: []string{"file1.txt", "file2.go", "file3.md"},
		},
		{
			name:     "duplicate references",
			content:  "Review @file.txt multiple times: @file.txt again",
			expected: []string{"file.txt"}, // Duplicates removed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFileReferences(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseFileReferences_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "Windows path separators",
			content:  "Load @path\\to\\file.txt",
			expected: []string{"path\\to\\file.txt"},
		},
		{
			name:     "filename with dots",
			content:  "Process @my.file.name.txt",
			expected: []string{"my.file.name.txt"},
		},
		{
			name:     "reference in quoted string",
			content:  "Process \"@file.txt\"",
			expected: []string{"file.txt"},
		},
		{
			name:     "at sign in email",
			content:  "Email user@example.com about @file.txt",
			expected: []string{"example.com", "file.txt"}, // Both match as file references
		},
		{
			name:     "at sign followed by space (no match)",
			content:  "Reference @ file.txt",
			expected: []string{}, // Space after @ prevents match
		},
		{
			name:     "multiple at signs",
			content:  "@file1 @file2 @file3",
			expected: []string{"file1", "file2", "file3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFileReferences(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

