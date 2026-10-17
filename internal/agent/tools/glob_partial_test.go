package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// helperSh emits a fixed number of NUL-terminated paths, writes to stderr, then
// exits with the given code — standing in for a ripgrep run that dies partway
// through a listing.
const helperSh = `#!/bin/sh
i=1
while [ $i -le $1 ]; do
  printf 'file%s.go\0' "$i"
  i=$((i + 1))
done
printf 'boom' >&2
exit $2
`

func writeHelper(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell helper")
	}
	path := filepath.Join(t.TempDir(), "fake-rg.sh")
	require.NoError(t, os.WriteFile(path, []byte(helperSh), 0o755))
	return path
}

func TestRunRipgrepReportsNoMatchesOnExitOne(t *testing.T) {
	t.Parallel()

	helper := writeHelper(t)
	matches, partial, err := runRipgrep(exec.Command(helper, "0", "1"), t.TempDir(), 100) //nolint:gosec // test helper
	require.NoError(t, err)
	require.Empty(t, matches)
	require.False(t, partial, "no matches is a complete answer, not a partial one")
}

func TestRunRipgrepMarksListingPartialWhenRipgrepFailsMidStream(t *testing.T) {
	t.Parallel()

	helper := writeHelper(t)
	// Two paths, then exit code 2 — a real ripgrep failure (bad flag, unreadable dir).
	matches, partial, err := runRipgrep(exec.Command(helper, "2", "2"), t.TempDir(), 100) //nolint:gosec // test helper

	require.NoError(t, err, "a partial listing must not be surfaced as an error (#2816)")
	require.Len(t, matches, 2, "the paths that were read are still returned")
	require.True(t, partial,
		"ripgrep died mid-listing, so the result must be marked incomplete rather than complete")
}

func TestRunRipgrepErrorsWhenNothingWasRead(t *testing.T) {
	t.Parallel()

	helper := writeHelper(t)
	// No output at all and a non-1 exit: nothing useful came back, so this is an error.
	_, _, err := runRipgrep(exec.Command(helper, "0", "2"), t.TempDir(), 100) //nolint:gosec // test helper
	require.Error(t, err)
	require.ErrorContains(t, err, "boom", "stderr should reach the caller")
}

func TestRunRipgrepMarksPartialWhenTheSearchIsKilled(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell helper")
	}
	script := `i=1
while true; do
  printf 'file%s.go\0' "$i"
  i=$((i + 1))
  sleep 0.05
done`
	path := filepath.Join(t.TempDir(), "slow-rg.sh")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	matches, partial, err := runRipgrep(
		exec.CommandContext(ctx, "/bin/sh", path), t.TempDir(), 100, //nolint:gosec // test helper
	)
	require.NoError(t, err)
	require.NotEmpty(t, matches)
	require.True(t, partial,
		"a killed ripgrep run leaves an incomplete listing, which must not look complete")
}

func TestRunRipgrepCompleteListingIsNotMarkedPartial(t *testing.T) {
	t.Parallel()

	helper := writeHelper(t)
	// Emits everything then exits 0: a complete answer.
	matches, partial, err := runRipgrep(exec.Command(helper, "3", "0"), t.TempDir(), 100) //nolint:gosec // test helper
	require.NoError(t, err)
	require.Len(t, matches, 3)
	require.False(t, partial, "a successful full listing is complete")
}
