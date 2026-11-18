package completions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFilesListsHostOpenFilesFirst(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o600))
	}

	open := []string{filepath.Join(dir, "c.go"), filepath.Join(dir, "gone.go"), "b.go", filepath.Join(filepath.Dir(dir), "outside.go")}
	files := loadFiles(0, 0, open)

	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	require.Equal(t, []string{"c.go", "b.go", "a.go"}, paths)
}
