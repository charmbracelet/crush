package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatModifiedFilePathUsesProjectRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	filePath := filepath.Join(root, "internal", "agent", "main.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte("package main"), 0o644))

	display := formatModifiedFilePath(filepath.Join(root, "dist"), filePath)
	require.Equal(t, filepath.Join("internal", "agent", "main.go"), display)
}

func TestCompactModifiedFilePathKeepsTail(t *testing.T) {
	t.Parallel()

	path := filepath.Join("internal", "verylongmodule", "subsystem", "main.go")
	compact := compactModifiedFilePath(path, 24)
	require.Contains(t, compact, filepath.Join("subsystem", "main.go"))
	require.NotContains(t, compact, "~")
}
