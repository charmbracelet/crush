package prompt

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func loadContents(t *testing.T, paths ...string) []string {
	t.Helper()
	store := config.NewTestStore(&config.Config{})
	files := loadContextFiles(paths, store)
	var out []string
	for _, group := range files {
		for _, file := range group {
			out = append(out, file.Content)
		}
	}
	slices.Sort(out)
	return out
}

func TestLoadContextFilesSymlinkIncludedOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	agents := filepath.Join(dir, "AGENTS.md")
	require.NoError(t, os.WriteFile(agents, []byte("CANARY-ONE"), 0o644))
	claude := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.Symlink(agents, claude))

	require.Equal(t, []string{"CANARY-ONE"}, loadContents(t, claude, agents))
	require.Equal(t, []string{"CANARY-ONE"}, loadContents(t, agents, claude))
}

func TestLoadContextFilesMissingCandidateDoesNotSuppressLaterFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lower := filepath.Join(dir, "agents.md")
	require.NoError(t, os.WriteFile(lower, []byte("CANARY-TWO"), 0o644))
	missing := filepath.Join(dir, "AGENTS.md")
	if _, err := os.Lstat(missing); err == nil {
		t.Skip("filesystem treats the missing candidate as the existing lowercase path")
	}

	require.Equal(t, []string{"CANARY-TWO"}, loadContents(t, missing, lower))
}

func TestLoadContextFilesDistinctCasePathsBothLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	upper := filepath.Join(dir, "AGENTS.md")
	lower := filepath.Join(dir, "agents.md")
	require.NoError(t, os.WriteFile(upper, []byte("UPPER"), 0o644))
	if err := os.WriteFile(lower, []byte("LOWER"), 0o644); err != nil {
		t.Skip("filesystem rejected a distinct lowercase path")
	}
	upperInfo, err := os.Stat(upper)
	require.NoError(t, err)
	lowerInfo, err := os.Stat(lower)
	require.NoError(t, err)
	if os.SameFile(upperInfo, lowerInfo) {
		t.Skip("filesystem treats the two names as one file")
	}

	require.Equal(t, []string{"LOWER", "UPPER"}, loadContents(t, upper, lower))
}
