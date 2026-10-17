package gitutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
	gitArgs := append([]string{"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)
	cmd := exec.CommandContext(context.Background(), "git", gitArgs...)
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

		branch := CurrentBranch(testDir)
		require.Empty(t, branch)
	})

	t.Run("returns empty string for non-git directory", func(t *testing.T) {
		testDir := t.TempDir()

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

		branch := CurrentBranch(subDir)
		require.Equal(t, "develop", branch)
	})

	t.Run("ignores unusual branch config entries", func(t *testing.T) {
		testDir := t.TempDir()
		runGit(t, testDir, "init")
		runGit(t, testDir, "config", "user.email", "test@test.com")
		runGit(t, testDir, "config", "user.name", "Test User")

		testFile := filepath.Join(testDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))
		runGit(t, testDir, "add", ".")
		runGit(t, testDir, "commit", "-m", "initial commit")
		runGit(t, testDir, "checkout", "-b", "odd-config")

		// Some repositories have branch entries that point at non-branch
		// refs, which config parsers can reject outright.
		runGit(t, testDir, "config", "branch.odd-config.merge", "refs/pull/1234/head")

		branch := CurrentBranch(testDir)
		require.Equal(t, "odd-config", branch)
	})

	t.Run("follows gitdir pointer in worktree", func(t *testing.T) {
		root := t.TempDir()
		gitDir := filepath.Join(root, "repo", ".git", "worktrees", "wt")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(gitDir, "HEAD"),
			[]byte("ref: refs/heads/worktree-branch\n"),
			0o644,
		))

		worktree := filepath.Join(root, "wt")
		require.NoError(t, os.MkdirAll(worktree, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(worktree, ".git"),
			[]byte("gitdir: ../repo/.git/worktrees/wt\n"),
			0o644,
		))

		branch := CurrentBranch(worktree)
		require.Equal(t, "worktree-branch", branch)
	})
}

func TestRepoRootRejectsTraversal(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(repo, ".git"), 0o755))
	sub := filepath.Join(repo, "pkg", "a")
	dotted := filepath.Join(repo, "a..b")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.MkdirAll(dotted, 0o755))
	repo, err := filepath.EvalSymlinks(repo)
	require.NoError(t, err)
	sub = filepath.Join(repo, "pkg", "a")
	dotted = filepath.Join(repo, "a..b")

	cases := []struct {
		name string
		dir  string
		want string
	}{
		{name: "repository root", dir: repo, want: repo},
		{name: "nested directory", dir: sub, want: repo},
		{name: "name containing two dots", dir: dotted, want: repo},
		{name: "empty", dir: "", want: ""},
		{name: "climbs back into the repo", dir: sub + "/../..", want: ""},
		{name: "climbs out of the repo", dir: sub + "/../../..", want: ""},
		{name: "relative parent", dir: "..", want: ""},
		{name: "relative climb", dir: "../../etc", want: ""},
		{name: "backslash climb", dir: `pkg\..\..\etc`, want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, RepoRoot(tc.dir))
		})
	}
}
