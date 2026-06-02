package critic

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeDiff_EmptyPaths(t *testing.T) {
	t.Parallel()
	d, truncated, err := ComputeDiff(nil, NewSnapshotStore(), nil, 1024)
	require.NoError(t, err)
	require.Empty(t, d)
	require.False(t, truncated)
}

func TestLibraryDiff(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "a.txt")
	require.NoError(t, os.WriteFile(p, []byte("line1\nline2\n"), 0o644))

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p}))

	require.NoError(t, os.WriteFile(p, []byte("line1\nmodified\n"), 0o644))

	after := map[string][]byte{p: []byte("line1\nmodified\n")}
	d, truncated, err := libraryDiff([]string{p}, ss, after, 0)
	require.NoError(t, err)
	require.Contains(t, d, "-line2")
	require.Contains(t, d, "+modified")
	require.False(t, truncated)
}

func TestIsBinary(t *testing.T) {
	t.Parallel()
	require.True(t, isBinary([]byte("hello\x00world")))
	require.False(t, isBinary([]byte("hello world")))
}

func TestComputeDiff_NewFile(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "new.txt")

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p}))

	require.NoError(t, os.WriteFile(p, []byte("new content"), 0o644))

	after := map[string][]byte{p: []byte("new content")}
	d, truncated, err := libraryDiff([]string{p}, ss, after, 0)
	require.NoError(t, err)
	require.Contains(t, d, "new content")
	require.False(t, truncated)
}

func TestGitDiff_SkipsInNonGitRepo(t *testing.T) {
	t.Parallel()

	// t.TempDir is not a git repo.
	_, err := gitDiff([]string{"foo.txt"})
	require.Error(t, err)
}

func TestLibraryDiff_BinaryFile(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "bin.dat")
	require.NoError(t, os.WriteFile(p, []byte("data\x00binary"), 0o644))

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p}))

	require.NoError(t, os.WriteFile(p, []byte("data\x00changed"), 0o644))

	after := map[string][]byte{p: []byte("data\x00changed")}
	d, truncated, err := libraryDiff([]string{p}, ss, after, 0)
	require.NoError(t, err)
	require.Contains(t, d, "Binary file")
	require.False(t, truncated)
}

func TestGitDiff_InsideGitRepo(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("hello\n"), 0o644))
	initGit(t, tmp)

	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmp))
	defer func() { _ = os.Chdir(oldWd) }()

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{filepath.Join(tmp, "a.txt")}))

	require.NoError(t, os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("world\n"), 0o644))

	after := map[string][]byte{filepath.Join(tmp, "a.txt"): []byte("world\n")}
	d, truncated, err := ComputeDiff([]string{filepath.Join(tmp, "a.txt")}, ss, after, 1024)
	require.NoError(t, err)
	require.True(t, strings.Contains(d, "-hello") || strings.Contains(d, "+world") || strings.Contains(d, "diff"))
	require.False(t, truncated)
}

func TestLibraryDiff_Truncation(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "a.txt")
	require.NoError(t, os.WriteFile(p, []byte("line1\nline2\n"), 0o644))

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p}))

	require.NoError(t, os.WriteFile(p, []byte("line1\nmodified\n"), 0o644))

	after := map[string][]byte{p: []byte("line1\nmodified\n")}
	d, truncated, err := libraryDiff([]string{p}, ss, after, 10)
	require.NoError(t, err)
	require.True(t, truncated)
	require.Contains(t, d, "... (diff truncated)")
}

func initGit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())
}
