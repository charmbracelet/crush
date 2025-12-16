package gitutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetCurrentBranch(t *testing.T) {
	t.Run("returns branch name for normal checkout", func(t *testing.T) {
		testDir := t.TempDir()
		gitDir := filepath.Join(testDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(gitDir, "HEAD"),
			[]byte("ref: refs/heads/main\n"),
			0o644,
		))

		branch := GetCurrentBranch(testDir)
		require.Equal(t, "main", branch)
	})

	t.Run("returns branch name for feature branch", func(t *testing.T) {
		testDir := t.TempDir()
		gitDir := filepath.Join(testDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(gitDir, "HEAD"),
			[]byte("ref: refs/heads/feature/git-branch-display\n"),
			0o644,
		))

		branch := GetCurrentBranch(testDir)
		require.Equal(t, "feature/git-branch-display", branch)
	})

	t.Run("returns empty string for detached HEAD", func(t *testing.T) {
		testDir := t.TempDir()
		gitDir := filepath.Join(testDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(gitDir, "HEAD"),
			[]byte("abc123def456789\n"),
			0o644,
		))

		branch := GetCurrentBranch(testDir)
		require.Empty(t, branch)
	})

	t.Run("returns empty string for non-git directory", func(t *testing.T) {
		testDir := t.TempDir()

		branch := GetCurrentBranch(testDir)
		require.Empty(t, branch)
	})

	t.Run("finds git directory from subdirectory", func(t *testing.T) {
		testDir := t.TempDir()
		gitDir := filepath.Join(testDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(gitDir, "HEAD"),
			[]byte("ref: refs/heads/develop\n"),
			0o644,
		))

		subDir := filepath.Join(testDir, "src", "internal", "pkg")
		require.NoError(t, os.MkdirAll(subDir, 0o755))

		branch := GetCurrentBranch(subDir)
		require.Equal(t, "develop", branch)
	})

	t.Run("handles missing HEAD file gracefully", func(t *testing.T) {
		testDir := t.TempDir()
		gitDir := filepath.Join(testDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))
		// No HEAD file created

		branch := GetCurrentBranch(testDir)
		require.Empty(t, branch)
	})
}

func TestParseHeadRef(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"main branch", "ref: refs/heads/main", "main"},
		{"feature branch", "ref: refs/heads/feature/test", "feature/test"},
		{"detached HEAD", "abc123def456", ""},
		{"empty content", "", ""},
		{"invalid format", "something unexpected", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseHeadRef(tt.content)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFindGitDir(t *testing.T) {
	t.Run("finds git dir in current directory", func(t *testing.T) {
		testDir := t.TempDir()
		gitDir := filepath.Join(testDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))

		result := findGitDir(testDir)
		require.Equal(t, gitDir, result)
	})

	t.Run("finds git dir in parent directory", func(t *testing.T) {
		testDir := t.TempDir()
		gitDir := filepath.Join(testDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))

		subDir := filepath.Join(testDir, "a", "b", "c")
		require.NoError(t, os.MkdirAll(subDir, 0o755))

		result := findGitDir(subDir)
		require.Equal(t, gitDir, result)
	})

	t.Run("returns empty for non-git directory", func(t *testing.T) {
		testDir := t.TempDir()

		result := findGitDir(testDir)
		require.Empty(t, result)
	})
}
