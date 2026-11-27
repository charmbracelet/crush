package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRgSearchArgsKeepDashPattern(t *testing.T) {
	args := rgSearchArgs("-needle", "/work", "")
	require.Equal(t, []string{"--json", "-H", "-n", "-0", "-e", "-needle", "/work"}, args)
}

func TestRipgrepDashPatternWithTrailingIgnoreFile(t *testing.T) {
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("rg is not in PATH")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("-needle\n"), 0o644))
	ignore := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.WriteFile(ignore, []byte("skip\n"), 0o644))

	// searchWithRipgrep appends --ignore-file after the path, so the pattern
	// cannot be protected with a bare "--".
	args := append(rgSearchArgs("-needle", dir, ""), "--ignore-file", ignore)
	cmd := exec.CommandContext(t.Context(), rgPath, args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	require.Contains(t, string(out), "-needle")
}
