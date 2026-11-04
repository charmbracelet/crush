package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// snapshotBaseTime is a fixed mtime for files whose modification time the
// tests control explicitly.
var snapshotBaseTime = time.Unix(1_700_000_000, 0)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// initGitRepo makes dir a git repository whose initial commit holds
// everything already in it.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
}

// newGitRepo returns a symlink-free temp repository with one committed file,
// tracked.txt.
func newGitRepo(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	writeFileAt(t, filepath.Join(repo, "tracked.txt"), "one", snapshotBaseTime)
	initGitRepo(t, repo)
	return repo
}

// writeFileAt writes content to path and pins its modification time, so tests
// do not depend on the clock resolution.
func writeFileAt(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	require.NoError(t, os.Chtimes(path, mtime, mtime))
}

func TestTakeWorktreeSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("outside a git repository", func(t *testing.T) {
		t.Parallel()
		snap, ok := takeWorktreeSnapshot(t.Context(), t.TempDir())
		require.False(t, ok)
		require.Nil(t, snap)
	})

	t.Run("clean repository", func(t *testing.T) {
		t.Parallel()
		snap, ok := takeWorktreeSnapshot(t.Context(), newGitRepo(t))
		require.True(t, ok)
		require.NotNil(t, snap)
	})
}

func TestChangedBetween(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		workDir string
		before  func(t *testing.T, repo string)
		change  func(t *testing.T, repo string)
		want    []string
	}{
		{
			name: "clean file modified",
			change: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, "tracked.txt"), "changed content", snapshotBaseTime.Add(time.Minute))
			},
			want: []string{"tracked.txt"},
		},
		{
			name: "dirty file modified again with the same size",
			before: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, "tracked.txt"), "dirty-1", snapshotBaseTime)
			},
			change: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, "tracked.txt"), "dirty-2", snapshotBaseTime.Add(time.Minute))
			},
			want: []string{"tracked.txt"},
		},
		{
			name: "new untracked file",
			change: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, "new.txt"), "new", snapshotBaseTime)
			},
			want: []string{"new.txt"},
		},
		{
			name: "deleted tracked file",
			change: func(t *testing.T, repo string) {
				require.NoError(t, os.Remove(filepath.Join(repo, "tracked.txt")))
			},
			want: []string{"tracked.txt"},
		},
		{
			name: "dirty file cleaned again",
			before: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, "tracked.txt"), "dirty", snapshotBaseTime)
			},
			change: func(t *testing.T, repo string) {
				runGit(t, repo, "checkout", "--", "tracked.txt")
			},
			want: []string{"tracked.txt"},
		},
		{
			name: "file changed and committed",
			change: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, "tracked.txt"), "committed", snapshotBaseTime.Add(time.Minute))
				runGit(t, repo, "add", "tracked.txt")
				runGit(t, repo, "commit", "-q", "-m", "change")
			},
			want: []string{"tracked.txt"},
		},
		{
			name: "unchanged dirty file",
			before: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, "tracked.txt"), "dirty", snapshotBaseTime)
			},
		},
		{
			name: "ignored file",
			before: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, ".gitignore"), "*.log\n", snapshotBaseTime)
				runGit(t, repo, "add", ".gitignore")
				runGit(t, repo, "commit", "-q", "-m", "ignore logs")
			},
			change: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, "debug.log"), "noise", snapshotBaseTime)
			},
		},
		{
			name:    "subdirectory working directory reports top-level paths",
			workDir: "sub",
			before: func(t *testing.T, repo string) {
				require.NoError(t, os.MkdirAll(filepath.Join(repo, "sub"), 0o755))
			},
			change: func(t *testing.T, repo string) {
				writeFileAt(t, filepath.Join(repo, "sub", "inner.txt"), "inner", snapshotBaseTime)
				writeFileAt(t, filepath.Join(repo, "top.txt"), "top", snapshotBaseTime)
			},
			want: []string{"sub/inner.txt", "top.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newGitRepo(t)
			dir := filepath.Join(repo, tt.workDir)
			if tt.before != nil {
				tt.before(t, repo)
			}
			before, ok := takeWorktreeSnapshot(t.Context(), dir)
			require.True(t, ok)

			if tt.change != nil {
				tt.change(t, repo)
			}
			after, ok := takeWorktreeSnapshot(t.Context(), dir)
			require.True(t, ok)

			want := make([]string, 0, len(tt.want))
			for _, rel := range tt.want {
				want = append(want, filepath.Join(repo, rel))
			}
			slices.Sort(want)
			got := changedBetween(t.Context(), before, after)
			if len(want) == 0 {
				require.Empty(t, got)
				return
			}
			require.Equal(t, want, got)
		})
	}
}

func TestChangedBetween_NilSnapshotsYieldNil(t *testing.T) {
	t.Parallel()
	snap, ok := takeWorktreeSnapshot(t.Context(), newGitRepo(t))
	require.True(t, ok)

	require.Nil(t, changedBetween(t.Context(), nil, snap))
	require.Nil(t, changedBetween(t.Context(), snap, nil))
	require.Nil(t, changedBetween(t.Context(), nil, nil))
}
