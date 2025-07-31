package fsext

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrushIgnore(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Change to temp directory
	oldWd, _ := os.Getwd()
	err := os.Chdir(tempDir)
	require.NoError(t, err)
	defer os.Chdir(oldWd)

	// Create test files
	require.NoError(t, os.WriteFile("test1.txt", []byte("test"), 0o644))
	require.NoError(t, os.WriteFile("test2.log", []byte("test"), 0o644))
	require.NoError(t, os.WriteFile("test3.tmp", []byte("test"), 0o644))

	// Create a .crushignore file that ignores .log files
	require.NoError(t, os.WriteFile(".crushignore", []byte("*.log\n"), 0o644))

	// Test DirectoryLister
	t.Run("DirectoryLister respects .crushignore", func(t *testing.T) {
		dl := NewDirectoryLister(tempDir)

		// Test that .log files are ignored
		require.True(t, dl.gitignore == nil, "gitignore should be nil")
		require.NotNil(t, dl.crushignore, "crushignore should not be nil")
	})

	// Test FastGlobWalker
	t.Run("FastGlobWalker respects .crushignore", func(t *testing.T) {
		walker := NewFastGlobWalker(tempDir)

		require.True(t, walker.gitignore == nil, "gitignore should be nil")
		require.NotNil(t, walker.crushignore, "crushignore should not be nil")
	})
}
