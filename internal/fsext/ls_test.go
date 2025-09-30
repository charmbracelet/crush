package fsext

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func chdir(t *testing.T, dir string) {
	original, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(dir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := os.Chdir(original)
		require.NoError(t, err)
	})
}

func TestListDirectory(t *testing.T) {
	tempDir := t.TempDir()
	chdir(t, tempDir)

	testFiles := map[string]string{
		"regular.txt":     "content",
		".hidden":         "hidden content",
		".gitignore":      ".*\n*.log\n",
		"subdir/file.go":  "package main",
		"subdir/.another": "more hidden",
		"build.log":       "build output",
	}

	for filePath, content := range testFiles {
		dir := filepath.Dir(filePath)
		if dir != "." {
			require.NoError(t, os.MkdirAll(dir, 0o755))
		}

		err := os.WriteFile(filePath, []byte(content), 0o644)
		require.NoError(t, err)
	}

	t.Run("no limit", func(t *testing.T) {
		files, truncated, err := ListDirectory(".", nil, -1, -1)
		require.NoError(t, err)
		require.False(t, truncated)

		require.ElementsMatch(t, []string{
			"./regular.txt",
			"./subdir/",
			"./subdir/.another",
			"./subdir/file.go",
		}, files)
	})
	t.Run("limit", func(t *testing.T) {
		files, truncated, err := ListDirectory(".", nil, -1, 2)
		require.NoError(t, err)
		require.True(t, truncated)
		require.ElementsMatch(t, []string{
			"./regular.txt",
			"./subdir/",
		}, files)
	})
}
