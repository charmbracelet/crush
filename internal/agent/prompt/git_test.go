package prompt

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// runGit runs a git command in dir and fails the test if it errors.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// newRepo creates a git repository in a fresh temp dir with one commit on
// the named branch and returns its path.
func newRepo(t *testing.T, branch string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial commit")
	runGit(t, dir, "checkout", "-b", branch)
	return dir
}

// The git block is assembled on whichever machine owns the workspace files.
// In client/server mode that is the server, because the agent, and with it
// prompt building, lives in the backend. Reading the branch off the local
// filesystem is therefore reading the same repository the shell commands in
// this block run against.
func TestGetGitStatusReportsBranch(t *testing.T) {
	t.Parallel()

	dir := newRepo(t, "feature/thing")

	out, err := getGitStatus(context.Background(), dir)
	require.NoError(t, err)
	require.Contains(t, out, "Current branch: feature/thing\n")
	require.Contains(t, out, "Status: clean\n")
	require.Contains(t, out, "initial commit")
}

// A detached HEAD names no branch, so the line is left out rather than
// reported as empty. Colocated jj repositories sit in this state all the
// time, which is why the rest of the block still has to be useful without
// it.
func TestGetGitStatusOmitsBranchWhenDetached(t *testing.T) {
	t.Parallel()

	dir := newRepo(t, "feature/thing")
	runGit(t, dir, "checkout", "--detach", "HEAD")

	out, err := getGitStatus(context.Background(), dir)
	require.NoError(t, err)
	require.NotContains(t, out, "Current branch:")
	require.Contains(t, out, "Status: clean\n")
	require.Contains(t, out, "initial commit")
}

// A worktree keeps its HEAD in .git/worktrees/<name>, reached through a
// "gitdir:" pointer in the worktree's .git file. Resolving that pointer is
// the difference between naming the branch and reporting nothing.
func TestGetGitStatusReadsWorktreeBranch(t *testing.T) {
	t.Parallel()

	dir := newRepo(t, "main-work")
	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, dir, "worktree", "add", "-b", "side-work", linked)

	out, err := getGitStatus(context.Background(), linked)
	require.NoError(t, err)
	require.Contains(t, out, "Current branch: side-work\n")
}
