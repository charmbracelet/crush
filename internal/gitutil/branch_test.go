package gitutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// helper to check if git is available.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// helper to run git commands in a directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(output))
}

func TestCurrentBranch(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git is not available")
	}

	t.Run("returns branch name for normal checkout", func(t *testing.T) {
		testDir := t.TempDir()
		runGit(t, testDir, "init")
		runGit(t, testDir, "config", "user.email", "test@test.com")
		runGit(t, testDir, "config", "user.name", "Test User")

		testFile := filepath.Join(testDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))
		runGit(t, testDir, "add", ".")
		runGit(t, testDir, "commit", "-m", "initial commit")

		// Ensure we're on 'main' regardless of default branch name.
		cmd := exec.CommandContext(context.Background(), "git", "branch", "--show-current")
		cmd.Dir = testDir
		out, _ := cmd.Output()
		currentBranch := string(out[:len(out)-1])
		if currentBranch != "main" {
			runGit(t, testDir, "checkout", "-b", "main")
		}

		resetCache()

		branch := CurrentBranch(testDir)
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

		resetCache()

		branch := CurrentBranch(testDir)
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

		cmd := exec.CommandContext(context.Background(), "git", "rev-parse", "HEAD")
		cmd.Dir = testDir
		output, err := cmd.Output()
		require.NoError(t, err)
		commitHash := string(output[:len(output)-1])

		runGit(t, testDir, "checkout", commitHash)

		resetCache()

		branch := CurrentBranch(testDir)
		require.Empty(t, branch)
	})

	t.Run("returns empty string for non-git directory", func(t *testing.T) {
		testDir := t.TempDir()

		resetCache()

		branch := CurrentBranch(testDir)
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

		resetCache()

		branch := CurrentBranch(subDir)
		require.Equal(t, "develop", branch)
	})

	t.Run("caches result within refresh interval", func(t *testing.T) {
		testDir := t.TempDir()
		runGit(t, testDir, "init")
		runGit(t, testDir, "config", "user.email", "test@test.com")
		runGit(t, testDir, "config", "user.name", "Test User")

		testFile := filepath.Join(testDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))
		runGit(t, testDir, "add", ".")
		runGit(t, testDir, "commit", "-m", "initial commit")
		runGit(t, testDir, "checkout", "-b", "cached-branch")

		resetCache()

		branch1 := CurrentBranch(testDir)
		require.Equal(t, "cached-branch", branch1)

		// Switch branch externally — cache should still return old value.
		runGit(t, testDir, "checkout", "-b", "other-branch")
		branch2 := CurrentBranch(testDir)
		require.Equal(t, "cached-branch", branch2)
	})
}

// resetCache clears the global cache for testing.
func resetCache() {
	cache.mu.Lock()
	cache.dir = ""
	cache.value = ""
	cache.lastRead = time.Time{}
	cache.mu.Unlock()
}
