package dialog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

func writeMemory(t *testing.T, memDir, slug, desc string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(memDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(memDir, slug+".md"),
		[]byte("---\ndescription: "+desc+"\n---\n\nbody"), 0o644))
}

// TestScanMemoryDirSortsBySlug pins the scan order against the index order.
//
// The dialog used to sort by the concatenated "slug: description" string with
// a hand-rolled bubble sort, while the memory tool sorts the index by slug
// alone. Because '-' (0x2D) and the digits (0x30-0x39) all sort below ':'
// (0x3A), any slug that is a prefix of another inverted between the two: the
// tool wrote "build" before "build-commands", the dialog the other way round.
// The dialog now sorts by slug, matching the index it is displaying.
func TestScanMemoryDirSortsBySlug(t *testing.T) {
	memDir := filepath.Join(t.TempDir(), "memory")
	// Insert in an order that a correct sort has to fix.
	for _, m := range [][2]string{
		{"build-commands", "how to build"},
		{"a1", "one"},
		{"build", "the build"},
		{"a", "alpha"},
	} {
		writeMemory(t, memDir, m[0], m[1])
	}

	got, err := ScanMemoryDir(memDir)
	require.NoError(t, err)
	slugs := make([]string, 0, len(got))
	for _, e := range got {
		slugs = append(slugs, e.slug)
	}
	require.Equal(t, []string{"a", "a1", "build", "build-commands"}, slugs)
}

// TestScanMemoryDirMatchesToolIndexOrder is the cross-check that the display
// order and the order written into MEMORY.md cannot drift again.
func TestScanMemoryDirMatchesToolIndexOrder(t *testing.T) {
	dataDir := t.TempDir()
	memDir := filepath.Join(dataDir, "memory")
	for _, m := range [][2]string{
		{"build-commands", "how to build"},
		{"build", "the build"},
		{"a1", "one"},
		{"a", "alpha"},
	} {
		writeMemory(t, memDir, m[0], m[1])
	}

	scanned, err := ScanMemoryDir(memDir)
	require.NoError(t, err)

	// Ask the tool package to write the index, then compare orders.
	require.NoError(t, tools.RegenerateMemoryIndex(t.Context(), dataDir))
	index, err := os.ReadFile(filepath.Join(memDir, "MEMORY.md"))
	require.NoError(t, err)

	var want string
	want = "# Memory index\n\n"
	for _, e := range scanned {
		want += "- " + e.slug + ": " + e.description + "\n"
	}
	require.Equal(t, want, string(index),
		"the dialog's display order and description parsing must match the index the tool writes")
}

func TestScanMemoryDirMissingDirectoryIsEmptyNotAnError(t *testing.T) {
	got, err := ScanMemoryDir(filepath.Join(t.TempDir(), "no-such-memory-dir"))
	require.NoError(t, err)
	require.Empty(t, got)
}

// TestScanMemoryDirSkipsTheIndexAndNonMarkdown keeps MEMORY.md and leftover
// atomic-write temp files out of the entry list.
func TestScanMemoryDirSkipsTheIndexAndNonMarkdown(t *testing.T) {
	memDir := filepath.Join(t.TempDir(), "memory")
	writeMemory(t, memDir, "real", "kept")
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte("# Memory index\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "MEMORY.md.123.tmp"), []byte("garbage"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "notes.txt"), []byte("nope"), 0o644))

	got, err := ScanMemoryDir(memDir)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "real", got[0].slug)
	require.Equal(t, "kept", got[0].description)
}
