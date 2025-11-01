package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessCommandContent_ArgumentSubstitution(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		args     []string
		expected string
	}{
		{
			name:     "no placeholders",
			content:  "Simple command content",
			args:     []string{},
			expected: "Simple command content",
		},
		{
			name:     "$ARGS substitution",
			content:  "Review PR $ARGS",
			args:     []string{"123", "high"},
			expected: "Review PR 123 high",
		},
		{
			name:     "$ARGUMENTS substitution",
			content:  "Review PR $ARGUMENTS",
			args:     []string{"123", "high"},
			expected: "Review PR 123 high",
		},
		{
			name:     "positional arguments",
			content:  "Review PR $1 with priority $2",
			args:     []string{"123", "high"},
			expected: "Review PR 123 with priority high",
		},
		{
			name:     "mixed $ARGS and positional",
			content:  "Review $ARGS with priority $2",
			args:     []string{"123", "high"},
			expected: "Review 123 high with priority high",
		},
		{
			name:     "mixed $ARGUMENTS and positional",
			content:  "Review $ARGUMENTS with priority $2",
			args:     []string{"123", "high"},
			expected: "Review 123 high with priority high",
		},
		{
			name:     "preserves file references with $ARGS",
			content:  "Review @file1.txt and @file2.go with args $ARGS",
			args:     []string{"test"},
			expected: "Review @file1.txt and @file2.go with args test",
		},
		{
			name:     "preserves file references with $ARGUMENTS",
			content:  "Review @file1.txt and @file2.go with args $ARGUMENTS",
			args:     []string{"test"},
			expected: "Review @file1.txt and @file2.go with args test",
		},
		{
			name:     "missing arguments",
			content:  "Use $1 and $3",
			args:     []string{"a", "b"},
			expected: "Use a and ",
		},
		{
			name:     "empty args with $ARGS",
			content:  "Process $ARGS",
			args:     []string{},
			expected: "Process ",
		},
		{
			name:     "empty args with $ARGUMENTS",
			content:  "Process $ARGUMENTS",
			args:     []string{},
			expected: "Process ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processCommandContent(tt.content, tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessCommandContent_PreservesFileReferences(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		args     []string
		contains []string // Strings that should be present in output
	}{
		{
			name:     "single file reference",
			content:  "Review @file.txt with args $1",
			args:     []string{"test"},
			contains: []string{"@file.txt"},
		},
		{
			name:     "multiple file references",
			content:  "Compare @file1.txt and @file2.go",
			args:     []string{},
			contains: []string{"@file1.txt", "@file2.go"},
		},
		{
			name:     "file references with arguments",
			content:  "Process @file.txt with $ARGS",
			args:     []string{"arg1", "arg2"},
			contains: []string{"@file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processCommandContent(tt.content, tt.args)
			for _, shouldContain := range tt.contains {
				assert.Contains(t, result, shouldContain)
			}
		})
	}
}

