package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/lock"
	"github.com/stretchr/testify/require"
)

// TestAtomicWriteFileIsAtomicUnderConcurrentReads backs the PR's "atomic
// index writes" claim with a test.
//
// MEMORY.md is read by the prompt layer at launch while the agent may be
// rewriting it. atomicWriteFile writes to a temp file and renames it into
// place so a reader observes either the whole old file or the whole new one,
// never a partial one. Replacing the temp+rename with a plain os.WriteFile
// makes this fail with hundreds of short reads, most of them zero-length.
//
// The property under test is strictly all-or-nothing visibility. A write that
// FAILS is a different concern -- on Windows os.Rename is denied while a
// reader holds the destination open, which renameWithRetry mitigates -- so a
// failed write is logged and skipped rather than failing the test. What must
// never happen, on any platform, is a reader seeing a partial file.
func TestAtomicWriteFileIsAtomicUnderConcurrentReads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, memoryIndexFileName)
	want := []byte(strings.Repeat("A", 1<<20)) // large enough that a non-atomic write is observably torn
	require.NoError(t, fsext.AtomicWriteFile(path, want, 0o644))

	stop := make(chan struct{})
	short := make(chan int, 1024)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			b, err := os.ReadFile(path)
			if err == nil && len(b) != len(want) {
				select {
				case short <- len(b):
				default:
				}
			}
			// A tiny pause so a contended rename has a window to land;
			// without it Windows denies every rename for the whole run.
			time.Sleep(time.Millisecond)
		}
	}()

	denied := 0
	for range 200 {
		if err := fsext.AtomicWriteFile(path, want, 0o644); err != nil {
			// Availability, not atomicity. See the doc comment.
			denied++
			continue
		}
	}
	close(stop)
	<-done

	if denied > 0 {
		t.Logf("%d of 200 writes were denied by the OS while a reader held the file open", denied)
	}
	if n := len(short); n > 0 {
		sizes := make([]int, 0, n)
		for range n {
			sizes = append(sizes, <-short)
		}
		max := 12
		if len(sizes) < max {
			max = len(sizes)
		}
		t.Fatalf("a concurrent reader observed %d partial writes (first sizes %v, want %d); "+
			"MEMORY.md writes must be atomic", n, sizes[:max], len(want))
	}
}

// TestDeleteMemoryHoldsTheMemoryDirLock pins that DeleteMemory serialises
// against the same lock file memory_write's save path takes.
//
// The memory dialog used to remove the file and rewrite MEMORY.md itself with
// no lock at all, so a UI-side delete racing an agent-side save meant
// whichever os.Rename landed last silently discarded the other's index
// update. Both paths now go through DeleteMemory.
func TestDeleteMemoryHoldsTheMemoryDirLock(t *testing.T) {
	dataDir := t.TempDir()
	memDir := filepath.Join(dataDir, "memory")
	require.NoError(t, os.MkdirAll(memDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "doomed.md"),
		[]byte(formatMemoryFile("doomed", "body")), 0o644))

	// Hold the lock another writer would hold.
	release, err := lock.File(context.Background(), filepath.Join(dataDir, "memory-write.lock"))
	require.NoError(t, err)

	errc := make(chan error, 1)
	go func() { errc <- DeleteMemory(context.Background(), dataDir, "doomed") }()

	select {
	case err := <-errc:
		t.Fatalf("DeleteMemory returned %v while the memory lock was held; it did not take the lock", err)
	case <-time.After(300 * time.Millisecond):
		// Correct: blocked on the lock.
	}

	require.FileExists(t, filepath.Join(memDir, "doomed.md"),
		"DeleteMemory must not remove anything before acquiring the lock")

	release()
	select {
	case err := <-errc:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("DeleteMemory did not complete after the lock was released")
	}
	require.NoFileExists(t, filepath.Join(memDir, "doomed.md"))
}

func TestDeleteMemoryReportsMissing(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "memory"), 0o755))
	err := DeleteMemory(context.Background(), dataDir, "nope")
	require.ErrorIs(t, err, ErrMemoryNotFound)
}

// TestRegenerateMemoryIndexStaysWithinPromptBudget covers the index writes the
// save path's budget check never reached.
//
// memory_write's save path renders the index, compares it to
// MaxMemoryIndexBytes and rolls the save back when it would not fit. Delete
// and rollback, though, call regenerateMemoryIndex unconditionally with no
// budget check. A memory directory can hold far more than maxMemoryFiles
// entries when populated out of band (the write tool can create files under
// .crush/memory directly), and deleting one entry from such a directory used
// to write a multi-megabyte MEMORY.md that the prompt layer can only read the
// first MaxMemoryIndexBytes of anyway.
func TestRegenerateMemoryIndexStaysWithinPromptBudget(t *testing.T) {
	memDir := filepath.Join(t.TempDir(), "memory")
	require.NoError(t, os.MkdirAll(memDir, 0o755))

	// Enough entries that the rendered index is many times the budget.
	desc := strings.Repeat("d", maxMemoryDescription)
	const n = 2000
	for i := range n {
		require.NoError(t, os.WriteFile(
			filepath.Join(memDir, fmt.Sprintf("entry-%05d.md", i)),
			[]byte(formatMemoryFile(desc, "body")), 0o644,
		))
	}

	rendered, err := renderMemoryIndex(memDir)
	require.NoError(t, err)
	require.Greater(t, len(rendered), MaxMemoryIndexBytes,
		"test setup must produce an over-budget index")

	require.NoError(t, regenerateMemoryIndex(memDir))

	written, err := os.ReadFile(filepath.Join(memDir, memoryIndexFileName))
	require.NoError(t, err)
	require.LessOrEqual(t, len(written), MaxMemoryIndexBytes,
		"regenerateMemoryIndex wrote %d bytes, past the %d-byte budget prompt.go injects",
		len(written), MaxMemoryIndexBytes)
	// Whole lines only: a half-written entry in the prompt is worse than none.
	require.True(t, strings.HasSuffix(string(written), "\n"),
		"the clamped index must end on a line boundary, got tail %q",
		string(written[max(0, len(written)-40):]))
}

// TestAtomicWriteFileGivesUpAndReturnsTheError pins the retry bound on the
// shared helper: a write that can never succeed must surface its error rather
// than spin, and the retry budget in fsext keeps the wait bounded.
func TestAtomicWriteFileGivesUpAndReturnsTheError(t *testing.T) {
	dir := t.TempDir()
	// Destination directory does not exist, so the rename can never succeed.
	dst := filepath.Join(dir, "no-such-dir", "dst")

	start := time.Now()
	err := fsext.AtomicWriteFile(dst, []byte("x"), 0o644)
	elapsed := time.Since(start)

	require.Error(t, err, "an impossible write must return its error")
	upper := 10 * time.Second
	require.Less(t, elapsed, upper,
		"AtomicWriteFile took %s; the retry loop must stay bounded", elapsed)
}
