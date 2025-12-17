package gitutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// helper to check if git is available
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// helper to run git commands in a directory
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(output))
}

func TestGetCurrentBranch(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git is not available")
	}

	ctx := context.Background()

	t.Run("returns branch name for normal checkout", func(t *testing.T) {
		testDir := t.TempDir()

		// Initialize a real git repository
		runGit(t, testDir, "init")
		runGit(t, testDir, "config", "user.email", "test@test.com")
		runGit(t, testDir, "config", "user.name", "Test User")

		// Create an initial commit (required for branch to exist)
		testFile := filepath.Join(testDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))
		runGit(t, testDir, "add", ".")
		runGit(t, testDir, "commit", "-m", "initial commit")

		// By default, git init creates 'master' or 'main' depending on config
		// Let's explicitly create and checkout 'main'
		runGit(t, testDir, "checkout", "-b", "main")

		branch := GetCurrentBranch(ctx, testDir)
		require.Equal(t, "main", branch)
	})

	t.Run("returns branch name for feature branch", func(t *testing.T) {
		testDir := t.TempDir()

		runGit(t, testDir, "init")
		runGit(t, testDir, "config", "user.email", "test@test.com")
		runGit(t, testDir, "config", "user.name", "Test User")

		testFile := filepath.Join(testDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))
		runGit(t, testDir, "add", ".")
		runGit(t, testDir, "commit", "-m", "initial commit")
		runGit(t, testDir, "checkout", "-b", "feature/git-branch-display")

		branch := GetCurrentBranch(ctx, testDir)
		require.Equal(t, "feature/git-branch-display", branch)
	})

	t.Run("returns empty string for detached HEAD", func(t *testing.T) {
		testDir := t.TempDir()

		runGit(t, testDir, "init")
		runGit(t, testDir, "config", "user.email", "test@test.com")
		runGit(t, testDir, "config", "user.name", "Test User")

		testFile := filepath.Join(testDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))
		runGit(t, testDir, "add", ".")
		runGit(t, testDir, "commit", "-m", "initial commit")

		// Get the commit hash and checkout to detach HEAD
		cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
		cmd.Dir = testDir
		output, err := cmd.Output()
		require.NoError(t, err)
		commitHash := string(output[:len(output)-1]) // remove newline

		runGit(t, testDir, "checkout", commitHash)

		branch := GetCurrentBranch(ctx, testDir)
		require.Empty(t, branch)
	})

	t.Run("returns empty string for non-git directory", func(t *testing.T) {
		testDir := t.TempDir()

		branch := GetCurrentBranch(ctx, testDir)
		require.Empty(t, branch)
	})

	t.Run("finds git directory from subdirectory", func(t *testing.T) {
		testDir := t.TempDir()

		runGit(t, testDir, "init")
		runGit(t, testDir, "config", "user.email", "test@test.com")
		runGit(t, testDir, "config", "user.name", "Test User")

		testFile := filepath.Join(testDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))
		runGit(t, testDir, "add", ".")
		runGit(t, testDir, "commit", "-m", "initial commit")
		runGit(t, testDir, "checkout", "-b", "develop")

		subDir := filepath.Join(testDir, "src", "internal", "pkg")
		require.NoError(t, os.MkdirAll(subDir, 0o755))

		branch := GetCurrentBranch(ctx, subDir)
		require.Equal(t, "develop", branch)
	})
}

func TestIsInsideWorkTree(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git is not available")
	}

	ctx := context.Background()

	t.Run("returns true for git repository", func(t *testing.T) {
		testDir := t.TempDir()
		runGit(t, testDir, "init")

		result := isInsideWorkTree(ctx, testDir)
		require.True(t, result)
	})

	t.Run("returns true for subdirectory of git repository", func(t *testing.T) {
		testDir := t.TempDir()
		runGit(t, testDir, "init")

		subDir := filepath.Join(testDir, "a", "b", "c")
		require.NoError(t, os.MkdirAll(subDir, 0o755))

		result := isInsideWorkTree(ctx, subDir)
		require.True(t, result)
	})

	t.Run("returns false for non-git directory", func(t *testing.T) {
		testDir := t.TempDir()

		result := isInsideWorkTree(ctx, testDir)
		require.False(t, result)
	})
}
