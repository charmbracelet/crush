package model

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

func TestLoadGitBranch(t *testing.T) {
	t.Parallel()

	t.Run("returns empty for non git dir", func(t *testing.T) {
		t.Parallel()

		ui := &UI{
			com: &common.Common{
				Workspace: &testWorkspace{workingDir: t.TempDir()},
			},
		}

		msg := ui.loadGitBranch()()
		result, ok := msg.(gitBranchLoadedMsg)
		require.True(t, ok)
		require.Empty(t, result.branch)
		require.Nil(t, ui.gitWatcher)
	})

	t.Run("returns branch for git repo and starts watcher", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		runGit(t, dir, "init")
		runGit(t, dir, "checkout", "-b", "feature-branch")

		ui := &UI{
			com: &common.Common{
				Workspace: &testWorkspace{workingDir: dir},
			},
		}

		msg := ui.loadGitBranch()()
		result, ok := msg.(gitBranchLoadedMsg)
		require.True(t, ok)
		require.Equal(t, "feature-branch", result.branch)
		require.NotNil(t, ui.gitWatcher)
		ui.gitWatcher.Close()
	})

	t.Run("returns empty for detached head", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		runGit(t, dir, "init")
		runGit(t, dir, "commit", "--allow-empty", "-m", "init")
		hash := runGit(t, dir, "rev-parse", "--short", "HEAD")
		runGit(t, dir, "checkout", hash)

		ui := &UI{
			com: &common.Common{
				Workspace: &testWorkspace{workingDir: dir},
			},
		}

		msg := ui.loadGitBranch()()
		result, ok := msg.(gitBranchLoadedMsg)
		require.True(t, ok)
		require.Empty(t, result.branch)
		require.NotNil(t, ui.gitWatcher)
		ui.gitWatcher.Close()
	})

	t.Run("watchGitBranch returns nil when no watcher", func(t *testing.T) {
		t.Parallel()

		ui := &UI{
			com: &common.Common{},
		}
		cmd := ui.watchGitBranch()
		require.Nil(t, cmd)
	})
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = []string{
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s: %s", args, string(out))
	return strings.TrimSpace(string(out))
}
