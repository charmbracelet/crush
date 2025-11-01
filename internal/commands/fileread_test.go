package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFileContents_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.go")
	require.NoError(t, os.WriteFile(file1, []byte("Content 1"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("Content 2"), 0o644))

	results := readFileContents([]string{file1, file2})

	require.Len(t, results, 2)
	assert.Equal(t, file1, results[0].Path)
	assert.Equal(t, "Content 1", results[0].Content)
	assert.Equal(t, file2, results[1].Path)
	assert.Equal(t, "Content 2", results[1].Content)
}

func TestReadFileContents_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	missingFile := filepath.Join(tmpDir, "missing.txt")

	results := readFileContents([]string{missingFile})

	require.Len(t, results, 1)
	assert.Equal(t, missingFile, results[0].Path)
	assert.Empty(t, results[0].Content)
}

func TestReadFileContents_IsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	results := readFileContents([]string{subDir})

	require.Len(t, results, 1)
	assert.Equal(t, subDir, results[0].Path)
	assert.Empty(t, results[0].Content)
}

func TestReadFileContents_MixedSuccessAndFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create one valid file
	validFile := filepath.Join(tmpDir, "valid.txt")
	require.NoError(t, os.WriteFile(validFile, []byte("Valid content"), 0o644))

	// Use a missing file
	missingFile := filepath.Join(tmpDir, "missing.txt")

	results := readFileContents([]string{validFile, missingFile})

	require.Len(t, results, 2)
	assert.Equal(t, validFile, results[0].Path)
	assert.Equal(t, "Valid content", results[0].Content)
	assert.Equal(t, missingFile, results[1].Path)
	assert.Empty(t, results[1].Content)
}

func TestReadFileContents_EmptyInput(t *testing.T) {
	results := readFileContents([]string{})
	assert.Empty(t, results)
}

func TestReadFileContents_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	largeFile := filepath.Join(tmpDir, "large.txt")

	// Create a file with some content (not actually large, but tests the read path)
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	require.NoError(t, os.WriteFile(largeFile, content, 0o644))

	results := readFileContents([]string{largeFile})

	require.Len(t, results, 1)
	assert.Equal(t, largeFile, results[0].Path)
	assert.Equal(t, string(content), results[0].Content)
}

func TestFileReadError_ErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		err     *FileReadError
		message string
	}{
		{
			name: "not found",
			err: &FileReadError{
				Path: "/path/to/file.txt",
				Type: ErrorTypeNotFound,
			},
			message: "file not found: /path/to/file.txt",
		},
		{
			name: "permission denied",
			err: &FileReadError{
				Path: "/path/to/file.txt",
				Type: ErrorTypePermissionDenied,
			},
			message: "permission denied reading file: /path/to/file.txt",
		},
		{
			name: "is directory",
			err: &FileReadError{
				Path: "/path/to/dir",
				Type: ErrorTypeIsDirectory,
			},
			message: "path is a directory, not a file: /path/to/dir",
		},
		{
			name: "access error",
			err: &FileReadError{
				Path: "/path/to/file.txt",
				Type: ErrorTypeAccess,
			},
			message: "cannot access file: /path/to/file.txt",
		},
		{
			name: "read error",
			err: &FileReadError{
				Path: "/path/to/file.txt",
				Type: ErrorTypeRead,
			},
			message: "error reading file: /path/to/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.message, tt.err.Error())
		})
	}
}

