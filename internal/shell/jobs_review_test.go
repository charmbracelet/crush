package shell

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// S1: scoping by path hides jobs whose working dir is relative or outside the workspace.
func TestJobsBelongToTheirWorkspaceRoot(t *testing.T) {
	m := newBackgroundShellManager()
	root := t.TempDir()
	outside := t.TempDir()
	rel, err := m.StartFor(context.Background(), JobOwner{SessionID: "s", Root: root}, "sub", nil, "sleep 30", "relative working_dir")
	if err == nil {
		t.Cleanup(func() { _ = m.Kill(rel.ID) })
	}
	out, err2 := m.StartFor(context.Background(), JobOwner{SessionID: "s", Root: root}, outside, nil, "sleep 30", "absolute outside")
	require.NoError(t, err2)
	t.Cleanup(func() { _ = m.Kill(out.ID) })
	in, err3 := m.StartFor(context.Background(), JobOwner{SessionID: "s", Root: root}, root, nil, "sleep 30", "inside")
	require.NoError(t, err3)
	t.Cleanup(func() { _ = m.Kill(in.ID) })
	listed := map[string]bool{}
	for _, j := range m.Jobs(root) {
		listed[j.Description] = true
	}
	t.Logf("jobs the workspace %q can see: %v (startErr relative=%v)", root, listed, err)
	require.True(t, listed["inside"])
	if rel != nil {
		require.Truef(t, listed["relative working_dir"], "job started with a relative working_dir is invisible/unkillable from its own workspace")
	}
	require.True(t, listed["absolute outside"], "job started with working_dir outside the workspace is invisible/unkillable from its own workspace")
}

// S2: nested workspaces see (and could kill) each other's jobs.
func TestNestedWorkspaceDoesNotSeeChildJobs(t *testing.T) {
	m := newBackgroundShellManager()
	parent := t.TempDir()
	child := parent + "/app"
	require.NoError(t, osMkdir(child))
	j, err := m.StartFor(context.Background(), JobOwner{SessionID: "child-ws-session", Root: child}, child, nil, "sleep 30", "child workspace job")
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Kill(j.ID) })
	seenByParent := false
	for _, x := range m.Jobs(parent) {
		if x.ID == j.ID {
			seenByParent = true
		}
	}
	t.Logf("workspace %q lists the job of nested workspace %q: %v", parent, child, seenByParent)
	require.False(t, seenByParent, "a job of workspace %q is visible to workspace %q", child, parent)
}

// S3: how much memory one finished job keeps.
func TestCappedBufferRetainedMemoryIsBounded(t *testing.T) {
	var b cappedBuffer
	chunk := make([]byte, 32<<10)
	for i := range chunk {
		chunk[i] = 'x'
	}
	for range 3200 { // ~100 MiB
		_, _ = b.Write(chunk)
	}
	t.Logf("after %d bytes written: len(head)=%d cap(head)=%d len(tail)=%d cap(tail)=%d", b.total(), len(b.head), cap(b.head), len(b.tail), cap(b.tail))
	perStream := cap(b.head) + cap(b.tail)
	t.Logf("retained per stream ~= %.2f MiB", float64(perStream)/(1<<20))
	require.LessOrEqual(t, perStream, 2*outputHeadCap+4*outputTailCap, "memory per stream stays bounded")
}

// S4: churn: Jobs() must never list an id twice, and never show running after it was seen finished.
func TestJobsChurnNoDuplicatesOrRegressions(t *testing.T) {
	m := newBackgroundShellManager()
	dir := t.TempDir()
	stop := make(chan struct{})
	var dupes, regress atomic.Int64
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			seenFinished := map[string]bool{} // per reader: only sequential calls are compared
			for {
				select {
				case <-stop:
					return
				default:
				}
				seen := map[string]int{}
				for _, j := range m.Jobs(dir) {
					seen[j.ID]++
					if seen[j.ID] > 1 {
						dupes.Add(1)
					}
					if j.Status != JobRunning {
						seenFinished[j.ID] = true
					} else if seenFinished[j.ID] {
						regress.Add(1)
					}
				}
			}
		}()
	}
	var starters sync.WaitGroup
	for range 4 {
		starters.Add(1)
		go func() {
			defer starters.Done()
			for range 40 {
				bg, err := m.StartFor(context.Background(), JobOwner{}, dir, nil, "sleep 0.01", "")
				if err != nil {
					t.Error(err)
					return
				}
				bg.Wait()
				if bg.ID[len(bg.ID)-1]%2 == 0 {
					_ = m.Remove(bg.ID)
				}
			}
		}()
	}
	starters.Wait()
	close(stop)
	wg.Wait()
	t.Logf("duplicates=%d running-after-finished=%d", dupes.Load(), regress.Load())
	require.Zero(t, dupes.Load())
	require.Zero(t, regress.Load())
}

// S5: a job being Kill()ed disappears from Jobs() while its process is still alive.
func TestKillKeepsJobListedUntilExit(t *testing.T) {
	m := newBackgroundShellManager()
	dir := t.TempDir()
	// the child ignores SIGINT, so the 2s SIGKILL escalation applies
	bg, err := m.StartFor(context.Background(), JobOwner{}, dir, nil, `sh -c 'trap "" INT; sleep 20'`, "stubborn")
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)
	killed := make(chan struct{})
	go func() { _ = m.Kill(bg.ID); close(killed) }()
	var missingWhileAlive, listedWhileKilling int
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-killed:
			deadline = time.Now()
		default:
		}
		alive := !bg.IsDone()
		found := false
		for _, j := range m.Jobs(dir) {
			if j.ID == bg.ID {
				found = true
			}
		}
		if alive && !found {
			missingWhileAlive++
		}
		if alive && found {
			listedWhileKilling++
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Logf("samples where the process was still alive: missing from Jobs()=%d listed=%d", missingWhileAlive, listedWhileKilling)
	require.Zero(t, missingWhileAlive, "a running job vanished from Jobs() while Kill waited for it")
}

func osMkdir(p string) error { return os.MkdirAll(p, 0o755) }

var _ = runtime.GOOS
var _ = fmt.Sprintf

// S6: one huge Write (a builtin that emits a big string at once) pins the whole backing array.
func TestSingleHugeWriteDoesNotPinMemory(t *testing.T) {
	var b cappedBuffer
	big := make([]byte, 64<<20)
	_, _ = b.Write(big)
	t.Logf("after one 64MiB write: len(head)=%d len(tail)=%d cap(tail)=%d (%.1f MiB retained)", len(b.head), len(b.tail), cap(b.tail), float64(cap(b.tail))/(1<<20))
	require.LessOrEqual(t, cap(b.tail), 4*outputTailCap+64, "tail backing array not shrunk after trimming")
}

// S7: while Kill() waits, JobOutput(id) of the job fails too.
func TestKillKeepsOutputReadableUntilExit(t *testing.T) {
	m := newBackgroundShellManager()
	dir := t.TempDir()
	bg, err := m.StartFor(context.Background(), JobOwner{}, dir, nil, `sh -c 'echo started; trap "" INT; sleep 20'`, "stubborn")
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)
	go func() { _ = m.Kill(bg.ID) }()
	time.Sleep(300 * time.Millisecond)
	_, outErr := m.JobOutput(bg.ID)
	t.Logf("process alive=%v JobOutput err=%v", !bg.IsDone(), outErr)
	require.NoError(t, outErr, "output of a job that is still dying must stay readable")
	<-bg.done
}

// S8: a finished foreground job is still listed (history) after 25 newer jobs, but Stop/JobOutput say "not found".
func TestListedFinishedJobStopIsNoOp(t *testing.T) {
	m := newBackgroundShellManager()
	dir := t.TempDir()
	first, err := m.StartFor(context.Background(), JobOwner{}, dir, nil, "echo first", "first")
	require.NoError(t, err)
	first.Wait()
	require.NoError(t, m.Remove(first.ID)) // what the bash tool does after a run-then-wait command
	for range 25 {
		bg, err := m.StartFor(context.Background(), JobOwner{}, dir, nil, "true", "")
		require.NoError(t, err)
		bg.Wait()
		require.NoError(t, m.Remove(bg.ID))
	}
	listed := false
	for _, j := range m.Jobs(dir) {
		if j.ID == first.ID {
			listed = true
		}
	}
	stopErr := m.Stop(context.Background(), first.ID)
	_, outErr := m.JobOutput(first.ID)
	t.Logf("listed=%v Stop err=%v JobOutput err=%v", listed, stopErr, outErr)
	require.True(t, listed)
	require.NoError(t, stopErr, "stopping a finished job is documented as a no-op")
}
