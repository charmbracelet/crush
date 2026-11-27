package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func testToolGlob() config.ToolGlob {
	return config.ToolGlob{}
}

func testToolGlobWithLocate() config.ToolGlob {
	return config.ToolGlob{
		UseLocate: ptrBool(true),
	}
}

func ptrBool(b bool) *bool {
	return &b
}

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

	got, _, err := globFiles(context.Background(), "a/**/*.go", root, root, testToolGlob(), 100)
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

	got, _, err := globFiles(context.Background(), "**/*.go", project, project, testToolGlob(), 100)
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

	got, truncated, err := globFiles(context.Background(), "**/*.go", root, root, testToolGlob(), 10)
	require.NoError(t, err)
	require.LessOrEqual(t, len(got), 10, "must not exceed limit")
	require.True(t, truncated, "should report truncation on an over-limit tree")
}

func TestIsOutsideWorkingDir(t *testing.T) {
	t.Parallel()

	// Get current directory for relative path tests
	cwd, _ := os.Getwd()

	tests := []struct {
		name       string
		searchPath string
		workingDir string
		expected   bool
	}{
		{
			name:       "same directory",
			searchPath: "/home/user/project",
			workingDir: "/home/user/project",
			expected:   false,
		},
		{
			name:       "subdirectory",
			searchPath: "/home/user/project/src",
			workingDir: "/home/user/project",
			expected:   false,
		},
		{
			name:       "parent directory",
			searchPath: "/home/user",
			workingDir: "/home/user/project",
			expected:   true,
		},
		{
			name:       "sibling directory",
			searchPath: "/home/user/other",
			workingDir: "/home/user/project",
			expected:   true,
		},
		{
			name:       "root directory",
			searchPath: "/",
			workingDir: "/home/user/project",
			expected:   true,
		},
		{
			name:       "relative path going up (resolved)",
			searchPath: filepath.Join(cwd, "..", ".."),
			workingDir: filepath.Join(cwd, "project", "src"),
			expected:   true,
		},
		{
			name:       "relative path staying in (resolved)",
			searchPath: filepath.Join(cwd, "project", "src"),
			workingDir: filepath.Join(cwd, "project"),
			expected:   false,
		},
		{
			name:       "empty search path (current dir, resolved)",
			searchPath: cwd,
			workingDir: cwd,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isOutsideWorkingDir(tt.searchPath, tt.workingDir)
			require.Equal(t, tt.expected, result, "isOutsideWorkingDir(%q, %q)", tt.searchPath, tt.workingDir)
		})
	}
}

// TestRunLocate tests the runLocate function if locate is available
func TestRunLocate(t *testing.T) {
	// Skip if locate is not available
	if _, err := exec.LookPath("locate"); err != nil {
		t.Skip("locate command not available")
	}

	t.Parallel()

	ctx := context.Background()
	pattern := "*.go"
	limit := 10

	matches, err := runLocate(ctx, pattern, limit)
	require.NoError(t, err)
	require.LessOrEqual(t, len(matches), limit, "should not exceed limit")

	// Verify all matches end with .go (case insensitive)
	for _, match := range matches {
		require.True(t, len(match) > 3, "match should have content")
		// locate -i is case insensitive, so check case-insensitively
		require.True(t, filepath.Ext(match) == ".go" || filepath.Ext(match) == ".GO" || filepath.Ext(match) == ".Go" || filepath.Ext(match) == ".gO",
			"match %q should be a .go file", match)
	}
}
