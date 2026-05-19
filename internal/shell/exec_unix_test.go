//go:build unix

package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestRun_CtxCancel_KillsProcessTree is the regression test for the
// motivating bug ("cancel leaves orphaned editors / dev servers
// running"). It exercises the exact shape the bug took:
//
//  1. A bare external command (no shebang dispatch, no path prefix) is
//     run via the standard handler chain.
//  2. That command forks a grandchild and waits on it.
//  3. The caller cancels the context.
//
// With process-group isolation the grandchild dies along with its
// parent. Without the fix, the kernel reparents the grandchild to PID 1
// when the parent is killed and the grandchild keeps running. We probe
// for liveness with kill(pid, 0) (ESRCH means gone) on a deadline that
// includes killTimeout plus a small wall-clock fudge for the reaper.
func TestRun_CtxCancel_KillsProcessTree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	scriptPath := filepath.Join(dir, "run.sh")

	// /bin/sh runs as our external command; the inner `sleep` is its
	// grandchild relative to the Crush process. Pre-fix, killing sh
	// would leave that sleep orphaned. Use 600 so a leaked process is
	// obviously a leak rather than racing the test's deadline.
	script := fmt.Sprintf("sleep 600 & echo $! > %q\nwait\n", pidfile)
	require := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	require(os.WriteFile(scriptPath, []byte(script), 0o600))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, RunOptions{
			Command: "/bin/sh " + strconv.Quote(scriptPath),
			Cwd:     dir,
			Env:     os.Environ(),
		})
	}()

	pid := waitForPIDFile(t, pidfile, 3*time.Second)

	cancel()

	select {
	case err := <-done:
		if err != nil && !IsInterrupt(err) {
			t.Fatalf("Run returned unexpected error: %v", err)
		}
	case <-time.After(killTimeout + 5*time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}

	deadline := time.Now().Add(killTimeout + 3*time.Second)
	for time.Now().Before(deadline) {
		if err := unix.Kill(pid, 0); errors.Is(err, unix.ESRCH) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("grandchild pid %d still alive %v after cancel; the process group fix did not reach it", pid, killTimeout+3*time.Second)
}

// TestRun_CtxCancel_ReturnsContextErr verifies that a cancelled external
// command surfaces ctx.Err() rather than a synthetic exit code. Callers
// downstream (the bash tool, the agent loop, IsInterrupt) rely on
// errors.Is(err, context.Canceled) to tell "user cancelled" apart from
// "program exited with a signal".
func TestRun_CtxCancel_ReturnsContextErr(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, RunOptions{
			Command: "sleep 60",
			Cwd:     t.TempDir(),
			Env:     os.Environ(),
		})
	}()

	// Give the sleep a chance to start before cancelling.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(killTimeout + 3*time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}

// waitForPIDFile polls path until it contains a positive integer or the
// timeout expires. The PID-file dance is the simplest way to learn the
// PID of a grandchild spawned by an external command without plumbing
// pipes through mvdan's interpreter.
func waitForPIDFile(t *testing.T, path string, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			s := strings.TrimSpace(string(data))
			if s != "" {
				n, err := strconv.Atoi(s)
				if err == nil && n > 0 {
					return n
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pidfile %s never contained a valid pid", path)
	return 0
}
