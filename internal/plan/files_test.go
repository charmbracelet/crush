package plan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesListAndRead(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	names, err := List(t.Context(), wd)
	require.NoError(t, err)
	require.Empty(t, names)
	dir := filepath.Join(wd, ".crush", "plans")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), 0o700))
	for _, name := range []string{"2026-01-old.txt", "2026-11-план with spaces.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("# План\n\nText\n"), 0o600))
	}
	outside := filepath.Join(wd, "secret")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link.md")))
	names, err = List(t.Context(), wd)
	require.NoError(t, err)
	require.Equal(t, []string{"2026-11-план with spaces.md", "2026-01-old.txt"}, names)
	content, err := Read(t.Context(), wd, names[0])
	require.NoError(t, err)
	require.Equal(t, "# План\n\nText\n", content)
	for _, name := range []string{"", ".", "..", "../secret", outside, `..\secret`, "link.md", "nested", "missing.md"} {
		_, err := Read(t.Context(), wd, name)
		require.Error(t, err, name)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = List(ctx, wd)
	require.ErrorIs(t, err, context.Canceled)
	_, err = Read(ctx, wd, names[0])
	require.ErrorIs(t, err, context.Canceled)
}
