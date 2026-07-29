package shell

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestBackgroundShellManager_Start(t *testing.T) {
	t.Skip("Skipping this until I figure out why its flaky")
	t.Parallel()

	ctx := t.Context()
	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	bgShell, err := manager.Start(ctx, workingDir, nil, "echo 'hello world'", "")
	if err != nil {
		t.Fatalf("failed to start background shell: %v", err)
	}

	if bgShell.ID == "" {
		t.Error("expected shell ID to be non-empty")
	}

	// Wait for the command to complete
	bgShell.Wait()

	stdout, stderr, done, err := bgShell.GetOutput()
	if !done {
		t.Error("expected shell to be done")
	}

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "hello world") {
		t.Errorf("expected stdout to contain 'hello world', got: %s", stdout)
	}

	if stderr != "" {
		t.Errorf("expected empty stderr, got: %s", stderr)
	}
}

func TestBackgroundShellManager_Get(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	bgShell, err := manager.Start(ctx, workingDir, nil, "echo 'test'", "")
	if err != nil {
		t.Fatalf("failed to start background shell: %v", err)
	}

	// Retrieve the shell
	retrieved, ok := manager.Get(bgShell.ID)
	if !ok {
		t.Error("expected to find the background shell")
	}

	if retrieved.ID != bgShell.ID {
		t.Errorf("expected shell ID %s, got %s", bgShell.ID, retrieved.ID)
	}

	// Clean up
	manager.Kill(bgShell.ID)
}

func TestBackgroundShellManager_Kill(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	// Start a long-running command
	bgShell, err := manager.Start(ctx, workingDir, nil, "sleep 10", "")
	if err != nil {
		t.Fatalf("failed to start background shell: %v", err)
	}

	// Kill it
	err = manager.Kill(bgShell.ID)
	if err != nil {
		t.Errorf("failed to kill background shell: %v", err)
	}

	// Verify it's no longer in the manager
	_, ok := manager.Get(bgShell.ID)
	if ok {
		t.Error("expected shell to be removed after kill")
	}

	// Verify the shell is done
	if !bgShell.IsDone() {
		t.Error("expected shell to be done after kill")
	}
}

func TestBackgroundShellManager_KillNonExistent(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()

	err := manager.Kill("non-existent-id")
	if err == nil {
		t.Error("expected error when killing non-existent shell")
	}
}

func TestBackgroundShell_IsDone(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	bgShell, err := manager.Start(ctx, workingDir, nil, "echo 'quick'", "")
	if err != nil {
		t.Fatalf("failed to start background shell: %v", err)
	}

	// Wait for the command to complete (Windows is slower to spin up).
	require.Eventually(t, bgShell.IsDone, 5*time.Second, 50*time.Millisecond, "expected shell to be done")

	// Clean up
	manager.Kill(bgShell.ID)
}

func TestBackgroundShell_WithBlockFuncs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	blockFuncs := []BlockFunc{
		CommandsBlocker([]string{"curl", "wget"}),
	}

	bgShell, err := manager.Start(ctx, workingDir, blockFuncs, "curl example.com", "")
	if err != nil {
		t.Fatalf("failed to start background shell: %v", err)
	}

	// Wait for the command to complete
	bgShell.Wait()

	stdout, stderr, done, execErr := bgShell.GetOutput()
	if !done {
		t.Error("expected shell to be done")
	}

	// The command should have been blocked
	output := stdout + stderr
	if !strings.Contains(output, "not allowed") && execErr == nil {
		t.Errorf("expected command to be blocked, got stdout: %s, stderr: %s, err: %v", stdout, stderr, execErr)
	}

	// Clean up
	manager.Kill(bgShell.ID)
}

func TestBackgroundShellManager_List(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping flacky test on windows")
	}

	t.Parallel()

	ctx := t.Context()
	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	// Start two shells
	bgShell1, err := manager.Start(ctx, workingDir, nil, "sleep 1", "")
	if err != nil {
		t.Fatalf("failed to start first background shell: %v", err)
	}

	bgShell2, err := manager.Start(ctx, workingDir, nil, "sleep 1", "")
	if err != nil {
		t.Fatalf("failed to start second background shell: %v", err)
	}

	ids := manager.List()

	// Check that both shells are in the list
	found1 := false
	found2 := false
	for _, id := range ids {
		if id == bgShell1.ID {
			found1 = true
		}
		if id == bgShell2.ID {
			found2 = true
		}
	}

	if !found1 {
		t.Errorf("expected to find shell %s in list", bgShell1.ID)
	}
	if !found2 {
		t.Errorf("expected to find shell %s in list", bgShell2.ID)
	}

	// Clean up
	manager.Kill(bgShell1.ID)
	manager.Kill(bgShell2.ID)
}

func TestBackgroundShellManager_KillAll(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	// Start multiple long-running shells
	shell1, err := manager.Start(ctx, workingDir, nil, "sleep 10", "")
	if err != nil {
		t.Fatalf("failed to start shell 1: %v", err)
	}

	shell2, err := manager.Start(ctx, workingDir, nil, "sleep 10", "")
	if err != nil {
		t.Fatalf("failed to start shell 2: %v", err)
	}

	shell3, err := manager.Start(ctx, workingDir, nil, "sleep 10", "")
	if err != nil {
		t.Fatalf("failed to start shell 3: %v", err)
	}

	// Verify shells are running
	if shell1.IsDone() || shell2.IsDone() || shell3.IsDone() {
		t.Error("shells should not be done yet")
	}

	// Kill all shells
	manager.KillAll(t.Context())

	// Verify all shells are done
	if !shell1.IsDone() {
		t.Error("shell1 should be done after KillAll")
	}
	if !shell2.IsDone() {
		t.Error("shell2 should be done after KillAll")
	}
	if !shell3.IsDone() {
		t.Error("shell3 should be done after KillAll")
	}

	// Verify they're removed from the manager
	if _, ok := manager.Get(shell1.ID); ok {
		t.Error("shell1 should be removed from manager")
	}
	if _, ok := manager.Get(shell2.ID); ok {
		t.Error("shell2 should be removed from manager")
	}
	if _, ok := manager.Get(shell3.ID); ok {
		t.Error("shell3 should be removed from manager")
	}

	// Verify list is empty (or doesn't contain our shells)
	ids := manager.List()
	for _, id := range ids {
		if id == shell1.ID || id == shell2.ID || id == shell3.ID {
			t.Errorf("shell %s should not be in list after KillAll", id)
		}
	}
}

func TestBackgroundShellManager_KillAll_Timeout(t *testing.T) {
	t.Parallel()

	// XXX: can't use synctest here - causes --race to trip.

	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	// Start a shell that traps signals and ignores cancellation.
	_, err := manager.Start(t.Context(), workingDir, nil, "trap '' TERM INT; sleep 60", "")
	require.NoError(t, err)

	// Short timeout to test the timeout path.
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	t.Cleanup(cancel)

	start := time.Now()
	manager.KillAll(ctx)

	elapsed := time.Since(start)

	// Must return promptly after timeout, not hang for 60 seconds.
	require.Less(t, elapsed, 2*time.Second)
}

func TestBackgroundShell_WaitContext_Completed(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	close(done)

	bgShell := &BackgroundShell{done: done}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	require.True(t, bgShell.WaitContext(ctx))
}

func TestBackgroundShell_WaitContext_Canceled(t *testing.T) {
	t.Parallel()

	bgShell := &BackgroundShell{done: make(chan struct{})}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.False(t, bgShell.WaitContext(ctx))
}

// TestBackgroundShellManager_ListJobsIsDeterministic pins that ListJobs
// returns a stable, oldest-first order.
//
// csync.Map.Seq2 ranges over a copy of the backing Go map, so an unsorted
// ListJobs returns a fresh random permutation on every call (measured: 5
// distinct orders in 8 consecutive calls with 6 jobs). The jobs dialog calls
// ListJobs from refresh() and hands the result to list.SetItems, which keeps
// the selected *index* — so a shuffle both reorders the rendered list and
// makes ctrl+x kill whichever job randomly landed under the cursor instead
// of the one the user highlighted.
//
// 20 jobs inserted in a shuffled order: well past the n=12 threshold below
// which Go's pdqsort falls back to insertion sort, and far too many for a
// random permutation to match the sorted order by chance.
func TestBackgroundShellManager_ListJobsIsDeterministic(t *testing.T) {
	t.Parallel()

	const n = 20
	manager := newBackgroundShellManager()
	base := time.Now()
	// Insert in an order unrelated to StartedAt so a no-op sort cannot pass.
	for _, i := range []int{7, 3, 19, 0, 12, 5, 17, 1, 9, 14, 2, 18, 6, 11, 4, 16, 8, 13, 10, 15} {
		id := fmt.Sprintf("%03X", i+1)
		manager.shells.Set(id, &BackgroundShell{
			ID:        id,
			Command:   "job " + id,
			StartedAt: base.Add(time.Duration(i) * time.Second),
		})
	}

	want := make([]string, 0, n)
	for i := range n {
		want = append(want, fmt.Sprintf("%03X", i+1))
	}

	// Repeat: a single call could coincidentally come back sorted.
	for range 20 {
		got := make([]string, 0, n)
		for _, job := range manager.ListJobs() {
			got = append(got, job.ID)
		}
		require.Equal(t, want, got,
			"ListJobs must be deterministic and oldest-first; a shuffled list makes the jobs dialog kill the wrong job")
	}
}

// TestBackgroundShellManager_ListJobsOrdersZeroStartedAtByID covers the tie
// path: shells with no StartedAt must still come back in a stable order
// rather than a random one.
func TestBackgroundShellManager_ListJobsOrdersZeroStartedAtByID(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	for _, id := range []string{"00E", "003", "011", "001", "00A", "007", "013", "005", "00C", "009", "002", "012", "006", "00B", "004", "010", "008", "00D", "00F"} {
		manager.shells.Set(id, &BackgroundShell{ID: id})
	}

	first := make([]string, 0, 19)
	for _, job := range manager.ListJobs() {
		first = append(first, job.ID)
	}
	require.True(t, slices.IsSorted(first), "tied StartedAt must fall back to ID order, got %v", first)
	for range 20 {
		got := make([]string, 0, 19)
		for _, job := range manager.ListJobs() {
			got = append(got, job.ID)
		}
		require.Equal(t, first, got, "ListJobs order must not change between calls")
	}
}

func TestSyncBufferUnderCap(t *testing.T) {
	t.Parallel()
	sb := &syncBuffer{headLimit: 64, tailLimit: 64}
	input := []byte("hello world")
	n, err := sb.Write(input)
	require.NoError(t, err)
	require.Equal(t, len(input), n)
	require.Equal(t, string(input), sb.String())
	require.False(t, sb.truncated)
}

func TestSyncBufferExactBoundary(t *testing.T) {
	t.Parallel()
	sb := &syncBuffer{headLimit: 8, tailLimit: 8}
	input := []byte("HEADHEADTAILTAIL") // 16 bytes
	n, err := sb.Write(input)
	require.NoError(t, err)
	require.Equal(t, len(input), n)
	require.Equal(t, string(input), sb.String())
	require.NotContains(t, sb.String(), "truncated")
}

func TestSyncBufferOverCapSingleWrite(t *testing.T) {
	t.Parallel()
	const head, tail = 32, 32
	sb := &syncBuffer{headLimit: head, tailLimit: tail}
	// 200 bytes: first 32 head, last 32 tail, middle truncated
	input := make([]byte, 200)
	for i := range input {
		input[i] = byte('A' + i%26)
	}
	// mark head/tail regions uniquely
	copy(input[:4], []byte("HEAD"))
	copy(input[len(input)-4:], []byte("TAIL"))

	n, err := sb.Write(input)
	require.NoError(t, err)
	require.Equal(t, len(input), n)

	out := sb.String()
	require.True(t, strings.HasPrefix(out, string(input[:head])))
	require.True(t, strings.HasSuffix(out, string(input[len(input)-tail:])))
	require.Contains(t, out, "bytes truncated")
	require.LessOrEqual(t, len(out), head+tail+80)
	require.True(t, sb.truncated)
}

func TestSyncBufferManySmallWrites(t *testing.T) {
	t.Parallel()
	const head, tail = 64, 64
	sb := &syncBuffer{headLimit: head, tailLimit: tail}
	var full []byte
	for i := range 300 {
		chunk := []byte{byte('a' + i%26)}
		full = append(full, chunk...)
		n, err := sb.Write(chunk)
		require.NoError(t, err)
		require.Equal(t, 1, n)
	}
	out := sb.String()
	require.True(t, strings.HasPrefix(out, string(full[:head])))
	require.True(t, strings.HasSuffix(out, string(full[len(full)-tail:])))
	require.Contains(t, out, "bytes truncated")
	require.LessOrEqual(t, len(out), head+tail+80)
}

func TestSyncBufferWriteString(t *testing.T) {
	t.Parallel()
	sb := &syncBuffer{headLimit: 8, tailLimit: 8}
	n, err := sb.WriteString("abcdefghijklmnopqr") // 18
	require.NoError(t, err)
	require.Equal(t, 18, n)
	out := sb.String()
	require.True(t, strings.HasPrefix(out, "abcdefgh"))
	require.True(t, strings.HasSuffix(out, "ijklmnopqr"[len("ijklmnopqr")-8:]))
	require.Contains(t, out, "bytes truncated")
}

func TestSyncBufferHeapBounded(t *testing.T) {
	// No t.Parallel: this reads process-global runtime.MemStats, so any other
	// test allocating concurrently lands inside the measurement window.
	sb := newSyncBuffer()  // 1 MiB + 1 MiB
	const total = 32 << 20 // 32 MiB
	const chunk = 64 << 10
	payload := make([]byte, chunk)
	for i := range payload {
		payload[i] = 'x'
	}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	written := 0
	for written < total {
		n, err := sb.Write(payload)
		require.NoError(t, err)
		require.Equal(t, len(payload), n)
		written += n
	}

	runtime.GC()
	runtime.ReadMemStats(&after)

	out := sb.String()
	require.Contains(t, out, "bytes truncated")

	// The deterministic invariant: whatever the process heap did, the buffer
	// itself retains at most head+tail regardless of how much was written.
	require.LessOrEqual(t, len(sb.head), defaultSyncBufferHeadBytes)
	require.LessOrEqual(t, sb.tailLen, defaultSyncBufferTailBytes)
	require.LessOrEqual(t, cap(sb.tail), defaultSyncBufferTailBytes)
	require.Equal(t, int64(total), sb.total)

	// Retained payload is ~2 MiB; allow generous overhead under GC noise.
	growth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	if growth < 0 {
		growth = 0
	}
	require.Less(t, growth, int64(8<<20), "heap grew by %d bytes after writing %d", growth, total)
	require.LessOrEqual(t, len(out), defaultSyncBufferHeadBytes+defaultSyncBufferTailBytes+128)
}

// TestSyncBufferTailWritesDoNotAllocate pins the tail to a fixed ring. A tail
// that is re-sliced per write allocates and copies tailLimit bytes on every
// write while holding the write lock, so draining a noisy child costs
// O(total output) rather than O(1) amortized.
func TestSyncBufferTailWritesDoNotAllocate(t *testing.T) {
	// No t.Parallel: testing.AllocsPerRun panics in a parallel test.
	sb := newSyncBuffer()
	chunk := make([]byte, 32<<10)
	// Fill the head and saturate the tail so every measured write takes the
	// steady-state path.
	for range 128 {
		sb.Write(chunk)
	}
	allocs := testing.AllocsPerRun(200, func() {
		sb.Write(chunk)
	})
	require.Zero(t, allocs, "steady-state tail write must not allocate, got %v allocs/op", allocs)
	require.True(t, sb.truncated)
	require.Equal(t, defaultSyncBufferTailBytes, sb.tailLen)
}

// TestSyncBufferSteadyStateDoesNotCreep asserts the retained window converges to
// the documented bound instead of growing slowly forever, and that both the head
// and the tail content actually survive (a one-sided length bound would also
// pass on an empty buffer).
func TestSyncBufferSteadyStateDoesNotCreep(t *testing.T) {
	// No t.Parallel: this allocates heavily and would otherwise land inside
	// TestSyncBufferHeapBounded's process-global MemStats window.
	const head, tail = 64, 64
	sb := &syncBuffer{headLimit: head, tailLimit: tail}
	first := strings.Repeat("H", head)
	sb.WriteString(first)
	for i := range 200000 {
		sb.WriteString(string(rune('a' + i%26)))
	}
	require.Equal(t, head, len(sb.head))
	require.Equal(t, tail, sb.tailLen)
	require.Equal(t, tail, len(sb.tail), "ring must stay exactly tailLimit bytes")

	// Cap holds AND content survived, in both directions.
	out := sb.String()
	require.True(t, strings.HasPrefix(out, first), "head content lost")
	require.LessOrEqual(t, len(out), head+tail+64)
	require.Greater(t, len(out), head+tail)
	require.Equal(t, 1, strings.Count(out, "bytes truncated"), "elision marker duplicated or lost")

	// The marker must account for exactly what was dropped.
	var reported int64
	_, err := fmt.Sscanf(out[strings.Index(out, "["):], "[%d bytes truncated]", &reported)
	require.NoError(t, err)
	require.Equal(t, sb.total-int64(head+tail), reported)
}

// TestSyncBufferDoesNotSplitRunes guards the head and tail truncation points
// against cutting a multi-byte rune in half, which renders as U+FFFD. PR #10
// shipped exactly this bug in addLog.
func TestSyncBufferDoesNotSplitRunes(t *testing.T) {
	t.Parallel()
	// headLimit 9 puts the boundary inside the 3-byte rune at offset 8;
	// tailLimit 8 puts the tail boundary inside a 3-byte rune too.
	sb := &syncBuffer{headLimit: 9, tailLimit: 8}
	sb.WriteString("abcdefgh世界" + strings.Repeat("Q", 12) + "世界世")
	out := sb.String()

	require.True(t, utf8.ValidString(out), "truncated output must remain valid UTF-8: %q", out)
	require.NotContains(t, out, string(utf8.RuneError), "truncation produced U+FFFD")
	require.True(t, strings.HasPrefix(out, "abcdefgh"), "head content lost")
	require.True(t, strings.HasSuffix(out, "界世"), "tail content lost: %q", out)

	// Accounting stays exact after trimming the rune fragments.
	var reported int64
	_, err := fmt.Sscanf(out[strings.Index(out, "["):], "[%d bytes truncated]", &reported)
	require.NoError(t, err)
	visible := int64(len(out)) - int64(len(fmt.Sprintf("\n\n... [%d bytes truncated] ...\n\n", reported)))
	require.Equal(t, sb.total, visible+reported, "marker must account for every dropped byte")
}

// TestSyncBufferZeroValueRetainsOutput guards against a syncBuffer constructed
// outside newSyncBuffer silently discarding every byte, which a zero headLimit
// and tailLimit would otherwise cause.
func TestSyncBufferZeroValueRetainsOutput(t *testing.T) {
	t.Parallel()
	var sb syncBuffer
	const msg = "this output must not vanish"
	n, err := sb.WriteString(msg)
	require.NoError(t, err)
	require.Equal(t, len(msg), n)
	require.Equal(t, msg, sb.String())
	require.False(t, sb.truncated)
}
