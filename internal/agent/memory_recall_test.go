package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/memory"
	"github.com/stretchr/testify/require"
)

func TestParseMemorySelectionResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "valid JSON array",
			input:    `["project/goal", "user/preferred-language"]`,
			expected: []string{"project/goal", "user/preferred-language"},
		},
		{
			name:     "wrapped in markdown",
			input:    "Here are the relevant memories:\n```json\n[\"key1\", \"key2\"]\n```",
			expected: []string{"key1", "key2"},
		},
		{
			name:     "empty array",
			input:    "[]",
			expected: nil,
		},
		{
			name:     "no JSON found",
			input:    "No relevant memories found.",
			expected: nil,
		},
		{
			name:     "deduplicates keys",
			input:    `["key1", "key1", "key2"]`,
			expected: []string{"key1", "key2"},
		},
		{
			name:     "respects max selected",
			input:    `["k1", "k2", "k3", "k4", "k5", "k6", "k7"]`,
			expected: []string{"k1", "k2", "k3", "k4", "k5"},
		},
		{
			name:     "skips empty keys",
			input:    `["key1", "", "key2"]`,
			expected: []string{"key1", "key2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseMemorySelectionResponse(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildMemoryManifest(t *testing.T) {
	t.Parallel()

	infos := []memory.MemoryFileInfo{
		{Key: "project/goal", Description: "Ship MVP", Type: "project", Tags: []string{"roadmap"}},
		{Key: "user/lang", Description: "Prefers Go", Type: "user"},
	}

	manifest := buildMemoryManifest(infos)
	require.Contains(t, manifest, "project/goal")
	require.Contains(t, manifest, "Ship MVP")
	require.Contains(t, manifest, "#roadmap")
	require.Contains(t, manifest, "user/lang")
	require.Contains(t, manifest, "Prefers Go")
}

func TestParseExtractedMemories(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []extractedMemory
	}{
		{
			name:     "valid memories",
			input:    `[{"key": "user/style", "description": "Concise code", "content": "User prefers concise code"}]`,
			expected: []extractedMemory{{Key: "user/style", Description: "Concise code", Content: "User prefers concise code"}},
		},
		{
			name:     "multiple memories",
			input:    `[{"key": "k1", "description": "d1", "content": "c1"}, {"key": "k2", "description": "d2", "content": "c2"}]`,
			expected: []extractedMemory{{Key: "k1", Description: "d1", Content: "c1"}, {Key: "k2", Description: "d2", Content: "c2"}},
		},
		{
			name:     "skips invalid entries",
			input:    `[{"key": "", "description": "d", "content": "c"}, {"key": "k", "description": "d", "content": ""}, {"key": "valid", "description": "desc", "content": "content"}]`,
			expected: []extractedMemory{{Key: "valid", Description: "desc", Content: "content"}},
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: []extractedMemory{},
		},
		{
			name:     "no JSON array",
			input:    "No memories to extract",
			expected: nil,
		},
		{
			name:     "default description",
			input:    `[{"key": "test", "content": "content"}]`,
			expected: []extractedMemory{{Key: "test", Description: "Extracted from conversation", Content: "content"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseExtractedMemories(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
