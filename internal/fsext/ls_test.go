package fsext

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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

	files, truncated, err := ListDirectory(".", nil, 0)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Equal(t, len(files), 4)

	fileSet := make(map[string]bool)
	for _, file := range files {
		fileSet[filepath.ToSlash(file)] = true
	}

	assert.True(t, fileSet["./regular.txt"])
	assert.True(t, fileSet["./subdir/"])
	assert.True(t, fileSet["./subdir/file.go"])
	assert.True(t, fileSet["./regular.txt"])

	assert.False(t, fileSet["./.hidden"])
	assert.False(t, fileSet["./.gitignore"])
	assert.False(t, fileSet["./build.log"])
}
