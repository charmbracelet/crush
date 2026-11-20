//go:build !windows

package shell

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Each test uses a distinct sleep duration so its pgrep/pkill guards only
// see its own descendants, even when the tests run in parallel.

// reapSleep removes leftover sleep processes started by a test.
func reapSleep(t *testing.T, duration string) {
	t.Helper()
	// context.Background, not t.Context: this runs after the test's
	// context is already cancelled.
	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "pkill", "-f", "sleep "+duration).Run()
	})
}

// requireDescendant waits until a process matching the pattern is alive, so
// the test kills a job whose descendant has definitely started.
func requireDescendant(t *testing.T, pattern string) {
	t.Helper()
	require.Eventually(t, func() bool {
		return exec.CommandContext(context.Background(), "pgrep", "-f", pattern).Run() == nil
	}, 5*time.Second, 50*time.Millisecond, "descendant %q never started", pattern)
}

// killWithin calls Kill and fails the test if it blocks past the deadline,
// rather than hanging until the package test timeout.
func killWithin(t *testing.T, manager *BackgroundShellManager, id string, limit time.Duration) {
	t.Helper()
	result := make(chan error, 1)
	start := time.Now()
	go func() { result <- manager.Kill(t.Context(), id) }()
	select {
	case err := <-result:
		require.NoError(t, err)
		require.Less(t, time.Since(start), limit, "Kill returned too slowly")
	case <-time.After(limit):
		t.Fatal("Kill hung: a descendant is holding the shell's pipes open")
	}
}

// A setsid'd grandchild leaves the job's process group and keeps the stdout
// pipe open after its parent exits. The exec handler's WaitDelay must unblock
// cmd.Wait so the job finishes and Kill returns.
func TestKill_SetsidDescendantHoldsPipe(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid not available")
	}
	reapSleep(t, "17")
	manager := newBackgroundShellManager()

	bgShell, err := manager.Start(context.Background(), JobScope{}, t.TempDir(), nil, "sh -c 'setsid sleep 17 &'", "", testMaxJobs)
	require.NoError(t, err)
	requireDescendant(t, "sleep 17")

	killWithin(t, manager, bgShell.ID, 6*time.Second)
	require.True(t, bgShell.IsDone())
}

// A backgrounded child that outlives its `sh -c` parent holds the output pipe
// open. Without a WaitDelay the job never completes on its own; with one it
// finishes about WaitDelay (killTimeout + 1s) after the parent exits.
func TestJobCompletesWhenBackgroundedChildHoldsPipe(t *testing.T) {
	reapSleep(t, "8")
	manager := newBackgroundShellManager()

	start := time.Now()
	bgShell, err := manager.Start(context.Background(), JobScope{}, t.TempDir(), nil, "sh -c 'sleep 8 &'; echo hi", "", testMaxJobs)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Second)
	t.Cleanup(cancel)
	require.True(t, bgShell.WaitContext(ctx), "job never completed: cmd.Wait was held open by the backgrounded child")
	require.Less(t, time.Since(start), 6*time.Second)
}

// The shebang dispatch path must kill the interpreter's whole process group,
// not just the interpreter PID, or the script's child keeps the pipes open
// and cmd.Wait blocks forever.
func TestKill_ShebangScriptChild(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "hang.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsleep 15 &\nwait\n"), 0o755))
	reapSleep(t, "15")
	manager := newBackgroundShellManager()

	bgShell, err := manager.Start(context.Background(), JobScope{}, dir, nil, "./hang.sh", "", testMaxJobs)
	require.NoError(t, err)
	requireDescendant(t, "sleep 15")

	killWithin(t, manager, bgShell.ID, 6*time.Second)
	require.True(t, bgShell.IsDone())
}
