package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFileAttachments_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.go")
	require.NoError(t, os.WriteFile(file1, []byte("Content 1"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("package main"), 0o644))

	fileContents := []FileContent{
		{Path: file1, Content: "Content 1"},
		{Path: file2, Content: "package main"},
	}

	attachments := buildFileAttachments(fileContents)

	require.Len(t, attachments, 2)
	assert.Equal(t, file1, attachments[0].FilePath)
	assert.Equal(t, "file1.txt", attachments[0].FileName)
	assert.Equal(t, "Content 1", string(attachments[0].Content))
	assert.NotEmpty(t, attachments[0].MimeType)

	assert.Equal(t, file2, attachments[1].FilePath)
	assert.Equal(t, "file2.go", attachments[1].FileName)
	assert.Equal(t, "package main", string(attachments[1].Content))
	assert.NotEmpty(t, attachments[1].MimeType)
}

func TestBuildFileAttachments_SkipsEmptyContent(t *testing.T) {
	fileContents := []FileContent{
		{Path: "/path/to/file1.txt", Content: "Valid content"},
		{Path: "/path/to/file2.txt", Content: ""}, // Empty - should be skipped
		{Path: "/path/to/file3.txt", Content: "Another valid"},
	}

	attachments := buildFileAttachments(fileContents)

	require.Len(t, attachments, 2)
	assert.Equal(t, "/path/to/file1.txt", attachments[0].FilePath)
	assert.Equal(t, "/path/to/file3.txt", attachments[1].FilePath)
}

func TestBuildFileAttachments_AllEmpty(t *testing.T) {
	fileContents := []FileContent{
		{Path: "/path/to/file1.txt", Content: ""},
		{Path: "/path/to/file2.txt", Content: ""},
	}

	attachments := buildFileAttachments(fileContents)

	assert.Empty(t, attachments)
}

func TestBuildFileAttachments_EmptyInput(t *testing.T) {
	attachments := buildFileAttachments([]FileContent{})
	assert.Empty(t, attachments)
}

func TestBuildFileAttachments_FileNameExtraction(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "simple filename",
			filePath: "/path/to/file.txt",
			expected: "file.txt",
		},
		{
			name:     "filename with extension",
			filePath: "/deep/nested/path/main.go",
			expected: "main.go",
		},
		{
			name:     "filename without extension",
			filePath: "/path/to/README",
			expected: "README",
		},
		{
			name:     "filename with dots",
			filePath: "/path/to/my.file.name.txt",
			expected: "my.file.name.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileContents := []FileContent{
				{Path: tt.filePath, Content: "test content"},
			}

			attachments := buildFileAttachments(fileContents)

			require.Len(t, attachments, 1)
			assert.Equal(t, tt.expected, attachments[0].FileName)
		})
	}
}

func TestDetectMimeType_TextFiles(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		content  string
		check    func(t *testing.T, mimeType string)
	}{
		{
			name:     "txt file",
			filePath: "file.txt",
			content:  "Plain text content",
			check: func(t *testing.T, mimeType string) {
				assert.Contains(t, mimeType, "text")
			},
		},
		{
			name:     "go file",
			filePath: "main.go",
			content:  "package main",
			check: func(t *testing.T, mimeType string) {
				// Should detect as text/plain or text/x-go
				assert.NotEmpty(t, mimeType)
			},
		},
		{
			name:     "json file",
			filePath: "data.json",
			content:  `{"key": "value"}`,
			check: func(t *testing.T, mimeType string) {
				// Content-based detection may return text/plain, but extension should work
				// Either way, should be non-empty
				assert.NotEmpty(t, mimeType)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mimeType := detectMimeType(tt.filePath, []byte(tt.content))
			tt.check(t, mimeType)
		})
	}
}

func TestBuildFileAttachments_TypeAssertion(t *testing.T) {
	// Verify that attachments are valid message.Attachment types
	fileContents := []FileContent{
		{Path: "/path/to/file.txt", Content: "test"},
	}

	attachments := buildFileAttachments(fileContents)

	require.Len(t, attachments, 1)
	// Type assertion - if this compiles, the type is correct
	var _ message.Attachment = attachments[0]
}

