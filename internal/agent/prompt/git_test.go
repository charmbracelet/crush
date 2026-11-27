package prompt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

// runGit runs a git command in dir and fails the test if it errors. Signing is
// disabled so a host configured to sign commits does not shell out to gpg.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	gitArgs := append([]string{"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)
	cmd := exec.CommandContext(context.Background(), "git", gitArgs...)
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

// binDirWith returns a directory holding symlinks to the named host
// executables, to be handed to a shell as its whole PATH. Naming an
// executable the host does not have is a test failure, so a skip would hide
// a broken assumption.
func binDirWith(t *testing.T, names ...string) string {
	t.Helper()

	dir := t.TempDir()
	for _, name := range names {
		path, err := exec.LookPath(name)
		require.NoError(t, err, "%s is not available", name)
		require.NoError(t, os.Symlink(path, filepath.Join(dir, name)))
	}
	return dir
}

// A snapshot that reports no changes is a claim the working tree is clean,
// and the model plans around it. A git that could not answer is not a git
// that answered "clean".
func TestGetGitStatusDoesNotClaimCleanWhenGitFails(t *testing.T) {
	t.Parallel()

	dir := newRepo(t, "feature/thing")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644))

	// head stays on PATH so the shell runs the pipeline; git does not.
	sh := shell.NewShell(&shell.Options{WorkingDir: dir, Env: []string{"PATH=" + binDirWith(t, "head")}})

	out, err := getGitStatusSummary(context.Background(), sh)
	require.NoError(t, err)
	require.NotContains(t, out, "Status: clean", "a failed git status must not be reported as a clean tree")
	require.Contains(t, out, "Status: unavailable")
}

// head is an external command, not a builtin, so a host without it (a
// Windows box outside Git Bash, a minimal container) has no status block at
// all when the status is asked for through a pipe.
func TestGetGitStatusWorksWithoutHeadOnPath(t *testing.T) {
	t.Parallel()

	dir := newRepo(t, "feature/thing")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644))

	sh := shell.NewShell(&shell.Options{WorkingDir: dir, Env: []string{"PATH=" + binDirWith(t, "git")}})

	out, err := getGitStatusSummary(context.Background(), sh)
	require.NoError(t, err)
	require.Contains(t, out, "Status:")
	require.Contains(t, out, "M a.txt")
}

// A dirty tree is listed, longest list capped so the prompt stays small.
func TestGetGitStatusListsChangesAndCapsThem(t *testing.T) {
	t.Parallel()

	dir := newRepo(t, "feature/thing")
	for i := range 25 {
		name := filepath.Join(dir, fmt.Sprintf("f%02d.txt", i))
		require.NoError(t, os.WriteFile(name, []byte("new"), 0o644))
	}

	out, err := getGitStatusSummary(context.Background(), shell.NewShell(&shell.Options{WorkingDir: dir}))
	require.NoError(t, err)
	require.Contains(t, out, "?? f00.txt")
	require.NotContains(t, out, "f24.txt", "status list is capped")
	require.Contains(t, out, "(+5 more)", "a capped list says how many entries were left out")
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
