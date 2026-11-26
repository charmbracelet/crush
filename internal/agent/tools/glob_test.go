package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlobFilesScopedPrefixMatchesUnscoped(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mkfile := func(rel string) {
		full := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("x"), 0o644))
	}
	mkfile("a/b/one.go")
	mkfile("a/b/c/two.go")
	mkfile("a/other.txt")
	mkfile("z/three.go")

	got, _, err := globFiles(context.Background(), "a/**/*.go", root, 100)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, p := range got {
		require.Contains(t, p, filepath.Join("a", "b"))
	}
}

func TestGlobFilesDoesNotFollowSymlinkEscape(t *testing.T) {
	t.Parallel()

	// Build a project dir with a symlink pointing outside it. With symlink
	// following disabled, the glob must not pick up files behind the link.
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.go"), []byte("x"), 0o644))

	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "in.go"), []byte("x"), 0o644))
	link := filepath.Join(project, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	got, _, err := globFiles(context.Background(), "**/*.go", project, 100)
	require.NoError(t, err)
	for _, p := range got {
		require.NotContains(t, p, "secret.go", "glob followed a symlink out of the search root")
	}
}

func TestGlobFilesCapsResultsOnLargeTree(t *testing.T) {
	t.Parallel()

	// A tree with far more matches than the limit must still return at
	// most `limit` results and report truncation, regardless of which
	// backend (ripgrep or the doublestar fallback) runs.
	root := t.TempDir()
	for i := range 500 {
		dir := filepath.Join(root, "pkg")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		name := filepath.Join(dir, "file"+string(rune('a'+i%26))+string(rune('a'+(i/26)%26))+"_"+string(rune('0'+i%10))+".go")
		require.NoError(t, os.WriteFile(name, []byte("x"), 0o644))
	}

	got, truncated, err := globFiles(context.Background(), "**/*.go", root, 10)
	require.NoError(t, err)
	require.LessOrEqual(t, len(got), 10, "must not exceed limit")
	require.True(t, truncated, "should report truncation on an over-limit tree")
}

func TestGlobFilesAbsolutePatternWalksWhereItPoints(t *testing.T) {
	t.Parallel()
	testGlobFilesAbsolutePattern(t)
}

func TestGlobFilesAbsolutePatternWithRipgrep(t *testing.T) {
	rg, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("ripgrep is not installed")
	}
	originalGetRg := getRg
	t.Cleanup(func() { getRg = originalGetRg })
	getRg = func() string { return rg }
	testGlobFilesAbsolutePattern(t)
}

func testGlobFilesAbsolutePattern(t *testing.T) {
	t.Helper()
	there, here := t.TempDir(), t.TempDir()
	target := filepath.Join(there, "etc")
	realFile := filepath.Join(target, "real.conf")
	relativeTarget := strings.TrimLeft(filepath.ToSlash(target[len(filepath.VolumeName(target)):]), "/")
	for _, path := range []string{
		realFile,
		filepath.Join(here, filepath.FromSlash(relativeTarget), "decoy.conf"),
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	}

	pattern := filepath.Join(target, "*.conf")
	got, _, err := globFiles(context.Background(), pattern, here, 100)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, realFile, filepath.Clean(got[0]))
}
