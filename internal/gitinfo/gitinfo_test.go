package gitinfo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGet_InGitRepo(t *testing.T) {
	t.Parallel()

	// Use the crush repo itself for testing.
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Navigate up to find repo root.
	repoRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			t.Skip("Not running inside a git repository")
		}
		repoRoot = parent
	}

	Invalidate() // Clear cache.
	info := Get(repoRoot)

	require.True(t, info.IsRepo)
	require.Equal(t, "crush", info.RepoName)
	require.NotEmpty(t, info.Branch)
	require.Empty(t, info.PathInRepo) // At root.

	// Test subdirectory.
	Invalidate()
	subdir := filepath.Join(repoRoot, "internal", "gitinfo")
	info = Get(subdir)

	require.True(t, info.IsRepo)
	require.Equal(t, "crush", info.RepoName)
	require.Equal(t, filepath.Join("internal", "gitinfo"), info.PathInRepo)
}

func TestGet_NotGitRepo(t *testing.T) {
	t.Parallel()

	Invalidate()
	info := Get(t.TempDir())

	require.False(t, info.IsRepo)
	require.Empty(t, info.Branch)
	require.Empty(t, info.RepoName)
}

func TestGet_Caching(t *testing.T) {
	t.Parallel()

	wd, err := os.Getwd()
	require.NoError(t, err)

	Invalidate()
	info1 := Get(wd)
	info2 := Get(wd) // Should be cached.

	require.Equal(t, info1.Branch, info2.Branch)
	require.Equal(t, info1.RepoName, info2.RepoName)
}
