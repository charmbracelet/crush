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
