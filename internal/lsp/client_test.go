package lsp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandlesFile(t *testing.T) {
	tests := []struct {
		name      string
		fileTypes []string
		filepath  string
		expected  bool
	}{
		{
			name:      "no file types specified - handles all files",
			fileTypes: nil,
			filepath:  "test.go",
			expected:  true,
		},
		{
			name:      "empty file types - handles all files",
			fileTypes: []string{},
			filepath:  "test.go",
			expected:  true,
		},
		{
			name:      "matches .go extension",
			fileTypes: []string{".go"},
			filepath:  "main.go",
			expected:  true,
		},
		{
			name:      "matches go extension without dot",
			fileTypes: []string{"go"},
			filepath:  "main.go",
			expected:  true,
		},
		{
			name:      "matches one of multiple extensions",
			fileTypes: []string{".js", ".ts", ".tsx"},
			filepath:  "component.tsx",
			expected:  true,
		},
		{
			name:      "does not match extension",
			fileTypes: []string{".go", ".rs"},
			filepath:  "script.sh",
			expected:  false,
		},
		{
			name:      "matches with full path",
			fileTypes: []string{".sh"},
			filepath:  "/usr/local/bin/script.sh",
			expected:  true,
		},
		{
			name:      "case insensitive matching",
			fileTypes: []string{".GO"},
			filepath:  "main.go",
			expected:  true,
		},
		{
			name:      "bash file types",
			fileTypes: []string{".sh", ".bash", ".zsh", ".ksh"},
			filepath:  "script.sh",
			expected:  true,
		},
		{
			name:      "bash should not handle go files",
			fileTypes: []string{".sh", ".bash", ".zsh", ".ksh"},
			filepath:  "main.go",
			expected:  false,
		},
		{
			name:      "bash should not handle json files",
			fileTypes: []string{".sh", ".bash", ".zsh", ".ksh"},
			filepath:  "config.json",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				fileTypes: tt.fileTypes,
			}
			result := client.HandlesFile(tt.filepath)
			require.Equal(t, tt.expected, result)
		})
	}
}
