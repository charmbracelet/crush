package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveFilePaths_RelativePaths(t *testing.T) {
	tmpDir := t.TempDir()
	workingDir := tmpDir

	tests := []struct {
		name      string
		filePaths []string
		expected  []string
	}{
		{
			name:      "simple filename",
			filePaths: []string{"file.txt"},
			expected:  []string{filepath.Join(workingDir, "file.txt")},
		},
		{
			name:      "path with subdirectory",
			filePaths: []string{"src/main.go"},
			expected:  []string{filepath.Join(workingDir, "src", "main.go")},
		},
		{
			name:      "nested path",
			filePaths: []string{"src/pkg/file.go"},
			expected:  []string{filepath.Join(workingDir, "src", "pkg", "file.go")},
		},
		{
			name:      "parent directory",
			filePaths: []string{"../parent/file.txt"},
			expected:  []string{filepath.Join(filepath.Dir(workingDir), "parent", "file.txt")},
		},
		{
			name:      "multiple relative paths",
			filePaths: []string{"file1.txt", "file2.go", "src/main.go"},
			expected: []string{
				filepath.Join(workingDir, "file1.txt"),
				filepath.Join(workingDir, "file2.go"),
				filepath.Join(workingDir, "src", "main.go"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveFilePaths(tt.filePaths, workingDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveFilePaths_AbsolutePaths(t *testing.T) {
	tmpDir := t.TempDir()
	workingDir := tmpDir

	tests := []struct {
		name      string
		filePaths []string
	}{
		{
			name:      "absolute Unix path",
			filePaths: []string{"/absolute/path/file.txt"},
		},
		{
			name:      "absolute Windows path",
			filePaths: []string{"C:\\absolute\\path\\file.txt"},
		},
		{
			name:      "mixed absolute and relative",
			filePaths: []string{"/absolute/file.txt", "relative/file.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveFilePaths(tt.filePaths, workingDir)

			// Check that absolute paths are preserved
			for i, filePath := range tt.filePaths {
				if filepath.IsAbs(filePath) {
					// Absolute path should be cleaned but preserved
					normalized := filepath.ToSlash(filePath)
					normalized = strings.ReplaceAll(normalized, "\\", "/")
					expected := filepath.Clean(normalized)
					assert.Equal(t, filepath.FromSlash(expected), result[i])
				} else {
					// Relative path should be resolved
					// Note: Windows absolute paths like "C:\\path" are not recognized as absolute on Unix
					// so they get treated as relative and joined with workingDir
					normalized := filepath.ToSlash(filePath)
					normalized = strings.ReplaceAll(normalized, "\\", "/")
					expected := filepath.Join(workingDir, filepath.Clean(normalized))
					assert.Equal(t, filepath.Clean(expected), result[i])
				}
			}
		})
	}
}

func TestResolveFilePaths_CrossPlatformSeparators(t *testing.T) {
	tmpDir := t.TempDir()
	workingDir := tmpDir

	tests := []struct {
		name      string
		filePaths []string
	}{
		{
			name:      "Windows separators",
			filePaths: []string{"path\\to\\file.txt"},
		},
		{
			name:      "Unix separators",
			filePaths: []string{"path/to/file.txt"},
		},
		{
			name:      "mixed separators",
			filePaths: []string{"path/to\\file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveFilePaths(tt.filePaths, workingDir)
			assert.Len(t, result, 1)

			// All paths should be normalized to use platform's separator
			assert.NotContains(t, result[0], "\\")
			if os.PathSeparator == '/' {
				assert.Contains(t, result[0], "/")
			}
		})
	}
}

func TestResolveFilePaths_EmptyInput(t *testing.T) {
	tmpDir := t.TempDir()
	workingDir := tmpDir

	result := resolveFilePaths([]string{}, workingDir)
	assert.Empty(t, result)
}

func TestResolveFilePaths_PathNormalization(t *testing.T) {
	tmpDir := t.TempDir()
	workingDir := tmpDir

	tests := []struct {
		name      string
		filePaths []string
		check     func(t *testing.T, result []string)
	}{
		{
			name:      "dot components",
			filePaths: []string{"./file.txt"},
			check: func(t *testing.T, result []string) {
				assert.Len(t, result, 1)
				assert.Equal(t, filepath.Join(workingDir, "file.txt"), result[0])
			},
		},
		{
			name:      "double slashes",
			filePaths: []string{"path//file.txt"},
			check: func(t *testing.T, result []string) {
				assert.Len(t, result, 1)
				assert.Equal(t, filepath.Join(workingDir, "path", "file.txt"), result[0])
			},
		},
		{
			name:      "trailing slash",
			filePaths: []string{"path/"},
			check: func(t *testing.T, result []string) {
				assert.Len(t, result, 1)
				// filepath.Clean removes trailing slashes
				assert.Equal(t, filepath.Join(workingDir, "path"), result[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveFilePaths(tt.filePaths, workingDir)
			tt.check(t, result)
		})
	}
}

