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

	dl := NewDirectoryLister(tempDir)
	require.True(t, dl.shouldIgnore("test2.log", nil), ".log files should be ignored")
	require.False(t, dl.shouldIgnore("test1.txt", nil), ".txt files should not be ignored")
	require.True(t, dl.shouldIgnore("test3.tmp", nil), ".tmp files should be ignored by common patterns")
}
