package fsext

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrushIgnoreGlobstarUnderscoreMarkdown(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	product := filepath.Join(tempDir, "product")
	require.NoError(t, os.MkdirAll(product, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(product, "_brain-dump.md"), []byte("secret"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(product, "normal.md"), []byte("ok"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".crushignore"), []byte("**/_*.md\n"), 0o644))

	brainDump := filepath.Join(product, "_brain-dump.md")
	normal := filepath.Join(product, "normal.md")

	require.True(t, ShouldExcludeFile(tempDir, brainDump), "expected _brain-dump.md to be ignored by **/_*.md")
	require.False(t, ShouldExcludeFile(tempDir, normal), "expected normal.md to remain visible")

	files, _, err := ListDirectory(tempDir, nil, 3, 100)
	require.NoError(t, err)

	for _, f := range files {
		require.NotContains(t, f, "_brain-dump.md", "ls should not list ignored _brain-dump.md, got: %v", files)
	}
	require.True(t, containsPath(files, normal), "ls should list normal.md, got: %v", files)
}

func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target || p == target+string(filepath.Separator) {
			return true
		}
	}
	return false
}
