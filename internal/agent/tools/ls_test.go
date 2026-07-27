package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestListDirectoryTreeNotices(t *testing.T) {
	t.Parallel()

	const (
		truncNotice = "There are more than"
		depthNotice = "shown up to a depth of"
	)

	// Wide shallow tree reliably trips MaxItems (fsext needs many sibling
	// entries so SkipAll fires while still under the depth cap).
	mkWide := func(t *testing.T, n int) string {
		t.Helper()
		root := t.TempDir()
		for i := range n {
			d := filepath.Join(root, fmt.Sprintf("d%02d", i))
			require.NoError(t, os.MkdirAll(d, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(d, "f.txt"), []byte("x"), 0o644))
		}
		return root
	}

	// Nested chain for depth-limit cases.
	mkDeep := func(t *testing.T, nestDepth int) string {
		t.Helper()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "top.txt"), []byte("x"), 0o644))
		cur := root
		for d := range nestDepth {
			cur = filepath.Join(cur, fmt.Sprintf("d%d", d))
			require.NoError(t, os.MkdirAll(cur, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(cur, "nested.txt"), []byte("x"), 0o644))
		}
		return root
	}

	ptr := func(v int) *int { return &v }

	t.Run("neither", func(t *testing.T) {
		t.Parallel()
		root := mkDeep(t, 1)
		out, meta, err := ListDirectoryTree(root, LSParams{}, config.ToolLs{})
		require.NoError(t, err)
		require.False(t, meta.Truncated)
		require.NotContains(t, out, truncNotice)
		require.NotContains(t, out, depthNotice)
	})

	t.Run("truncated only", func(t *testing.T) {
		t.Parallel()
		root := mkWide(t, 30)
		out, meta, err := ListDirectoryTree(root, LSParams{}, config.ToolLs{MaxItems: ptr(5)})
		require.NoError(t, err)
		require.True(t, meta.Truncated)
		require.Contains(t, out, truncNotice)
		require.NotContains(t, out, depthNotice)
	})

	t.Run("depth only via config", func(t *testing.T) {
		t.Parallel()
		root := mkDeep(t, 3)
		out, meta, err := ListDirectoryTree(root, LSParams{}, config.ToolLs{MaxDepth: ptr(1)})
		require.NoError(t, err)
		require.False(t, meta.Truncated)
		require.NotContains(t, out, truncNotice)
		require.Contains(t, out, depthNotice)
		require.Contains(t, out, "depth of 1")
	})

	t.Run("both truncated and depth", func(t *testing.T) {
		t.Parallel()
		root := mkWide(t, 30)
		out, meta, err := ListDirectoryTree(root, LSParams{}, config.ToolLs{
			MaxDepth: ptr(2),
			MaxItems: ptr(5),
		})
		require.NoError(t, err)
		require.True(t, meta.Truncated)
		require.Contains(t, out, truncNotice)
		require.Contains(t, out, depthNotice)
		require.Contains(t, out, "depth of 2")
	})

	t.Run("params.Depth with config depth 0", func(t *testing.T) {
		t.Parallel()
		root := mkDeep(t, 3)
		out, meta, err := ListDirectoryTree(root, LSParams{Depth: 2}, config.ToolLs{})
		require.NoError(t, err)
		require.False(t, meta.Truncated)
		require.NotContains(t, out, truncNotice)
		require.Contains(t, out, depthNotice)
		require.Contains(t, out, "depth of 2")
	})
}
