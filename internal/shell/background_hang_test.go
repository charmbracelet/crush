//go:build !windows

package shell

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

	bgShell, err := manager.Start(context.Background(), nil, t.TempDir(), nil, CoreutilsTurtle, "sh -c 'setsid sleep 17 &'", "", testMaxJobs)
	require.NoError(t, err)
	requireDescendant(t, "sleep 17")

	killWithin(t, manager, bgShell.ID, 6*time.Second)
	require.True(t, bgShell.IsDone())
}

// A backgrounded child that outlives its `sh -c` parent holds the output pipe
// open. Without a WaitDelay the job never completes on its own; with one it
// finishes about WaitDelay (killTimeout + 1s) after the parent exits, and
// the rest of the script still runs with a clean status.
func TestJobCompletesWhenBackgroundedChildHoldsPipe(t *testing.T) {
	reapSleep(t, "8")
	manager := newBackgroundShellManager()

	start := time.Now()
	bgShell, err := manager.Start(context.Background(), nil, t.TempDir(), nil, CoreutilsTurtle, "sh -c 'sleep 8 &'; echo hi", "", testMaxJobs)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Second)
	t.Cleanup(cancel)
	require.True(t, bgShell.WaitContext(ctx), "job never completed: cmd.Wait was held open by the backgrounded child")
	require.Less(t, time.Since(start), 6*time.Second)

	stdout, stderr, done, execErr := bgShell.GetOutput()
	require.True(t, done)
	require.NoError(t, execErr, "stderr=%q", stderr)
	require.Equal(t, 0, ExitCode(execErr))
	require.Equal(t, "hi\n", stdout)
}

// The shebang dispatch path has the same WaitDelay and must behave the same.
func TestRun_ShebangScriptContinuesWhenBackgroundedChildHoldsPipe(t *testing.T) {
	t.Parallel()
	reapSleep(t, "11")

	dir := t.TempDir()
	script := filepath.Join(dir, "bg.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsleep 11 &\n"), 0o755))

	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "./bg.sh; echo hi",
		Cwd:     dir,
		Env:     os.Environ(),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	require.NoError(t, err, "stderr=%q", stderr.String())
	require.Equal(t, "hi\n", stdout.String())
}

// Wait only reports ErrWaitDelay after a clean exit, so a failing command
// keeps its own status even while a backgrounded child holds the pipe.
func TestRun_NonZeroExitKeptWhenBackgroundedChildHoldsPipe(t *testing.T) {
	t.Parallel()
	reapSleep(t, "7")

	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "sh -c 'sleep 7 & exit 3'; echo rc=$?",
		Cwd:     t.TempDir(),
		Env:     os.Environ(),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	require.NoError(t, err, "stderr=%q", stderr.String())
	require.Equal(t, "rc=3\n", stdout.String())
}

// The shebang dispatch path must kill the interpreter's whole process group,
// not just the interpreter PID, or the script's child keeps the pipes open
// and cmd.Wait blocks forever. The child ignores SIGINT as a background job
// of a non-interactive shell, so this also covers the SIGKILL escalation.
func TestKill_ShebangScriptChild(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "hang.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsleep 15 &\necho $! > child.pid\nwait\n"), 0o755))
	reapSleep(t, "15")
	manager := newBackgroundShellManager()

	bgShell, err := manager.Start(context.Background(), nil, dir, nil, CoreutilsTurtle, "./hang.sh", "", testMaxJobs)
	require.NoError(t, err)

	var pid int
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(filepath.Join(dir, "child.pid"))
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
		return err == nil && pid > 1
	}, 5*time.Second, 10*time.Millisecond, "script never recorded its child PID")

	// The child survives SIGINT, so Kill must wait out the grace period
	// before the SIGKILL escalation takes it down.
	start := time.Now()
	killWithin(t, manager, bgShell.ID, defaultKillTimeout+4*time.Second)
	require.GreaterOrEqual(t, time.Since(start), defaultKillTimeout)
	require.True(t, bgShell.IsDone())

	// Poll until the child is gone; an orphaned zombie lingers briefly
	// until init reaps it.
	require.Eventually(t, func() bool {
		return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
	}, 5*time.Second, 20*time.Millisecond, "script child %d survived cancel", pid)
}
