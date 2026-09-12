package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestTruncateForPersist_KeepsSmallOutput(t *testing.T) {
	out := "\x1b[31mhello\x1b[0m\nworld\n"
	require.Equal(t, out, TruncateForPersist(out, ""))
}

func TestTruncateForPersist_SpillsLongOutput(t *testing.T) {
	var b strings.Builder
	for range 5000 {
		b.WriteString("commit ")
		b.WriteString(strings.Repeat("a", 20))
		b.WriteByte('\n')
	}
	full := b.String()

	got := TruncateForPersist(full, t.TempDir())
	require.Less(t, len(got), len(full))
	require.Contains(t, got, "lines omitted, full output: ")

	path := got[strings.LastIndex(got, "full output: ")+len("full output: "):]
	path = strings.TrimSpace(strings.SplitN(path, "\n", 2)[0])

	spilled, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, full, string(spilled))
}

func TestTruncateForPersist_StripsANSIAndBoundsBytes(t *testing.T) {
	line := "\x1b[32m" + strings.Repeat("x", 200) + "\x1b[0m"
	full := strings.Repeat(line+"\n", 3000)

	got := TruncateForPersist(full, t.TempDir())
	require.NotContains(t, got, "\x1b[")
	require.LessOrEqual(t, len(got), maxPersistedBytes+512)
}

func TestTruncateForPersist_SingleHugeLine(t *testing.T) {
	full := strings.Repeat("héllo", maxPersistedBytes)

	got := TruncateForPersist(full, t.TempDir())
	require.LessOrEqual(t, len(got), maxPersistedBytes+512)
	require.True(t, utf8.ValidString(got), "truncation must cut on rune boundaries")
}

func TestSpillOutput_PrunesOldFilesOnly(t *testing.T) {
	dataDir := t.TempDir()
	spillDir := filepath.Join(dataDir, spillSubdir)
	require.NoError(t, os.MkdirAll(spillDir, 0o700))

	stale := filepath.Join(spillDir, "output-stale.log")
	fresh := filepath.Join(spillDir, "output-fresh.log")
	keep := filepath.Join(spillDir, "notes.txt")
	for _, path := range []string{stale, fresh, keep} {
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	}
	old := time.Now().Add(-spillRetention - time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))
	require.NoError(t, os.Chtimes(keep, old, old))

	_, err := SpillOutput("hello", dataDir)
	require.NoError(t, err)

	require.NoFileExists(t, stale)
	require.FileExists(t, fresh)
	require.FileExists(t, keep, "pruning must not touch unrelated files")
}

func TestSpillOutput_LeavesTempDirAlone(t *testing.T) {
	bystander := filepath.Join(os.TempDir(), "output-not-ours.log")
	require.NoError(t, os.WriteFile(bystander, []byte("x"), 0o600))
	t.Cleanup(func() { os.Remove(bystander) })
	old := time.Now().Add(-spillRetention - time.Hour)
	require.NoError(t, os.Chtimes(bystander, old, old))

	path, err := SpillOutput("hello", "")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(path) })

	require.FileExists(t, bystander)
}

func TestTruncateForPersist_KeepsTailWhenLinesAreFewButLong(t *testing.T) {
	// Under the line budget but well over the byte budget: the end of the
	// output must survive, not just the first half.
	line := strings.Repeat("x", 500)
	var b strings.Builder
	for i := range 200 {
		fmt.Fprintf(&b, "%s-%03d\n", line, i)
	}
	full := b.String()

	got := TruncateForPersist(full, t.TempDir())
	require.Contains(t, got, "-000", "head must survive")
	require.Contains(t, got, "-199", "tail must survive")
}

func TestSpillOutput_EnforcesDirectoryLimit(t *testing.T) {
	dataDir := t.TempDir()
	spillDir := filepath.Join(dataDir, spillSubdir)
	require.NoError(t, os.MkdirAll(spillDir, 0o700))

	// Three fresh files, together over the cap. The oldest go first.
	chunk := make([]byte, spillDirLimit/2)
	names := []string{"output-1.log", "output-2.log", "output-3.log"}
	for i, name := range names {
		path := filepath.Join(spillDir, name)
		require.NoError(t, os.WriteFile(path, chunk, 0o600))
		age := time.Duration(len(names)-i) * time.Hour
		stamp := time.Now().Add(-age)
		require.NoError(t, os.Chtimes(path, stamp, stamp))
	}

	_, err := SpillOutput("hello", dataDir)
	require.NoError(t, err)

	require.NoFileExists(t, filepath.Join(spillDir, "output-1.log"), "oldest goes first")
	require.FileExists(t, filepath.Join(spillDir, "output-3.log"), "newest survives")
}
