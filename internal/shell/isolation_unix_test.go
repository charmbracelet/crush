//go:build !windows

package shell

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestProcessIsolation_SignalDoesNotReachParent verifies that a child
// sending SIGINT to its own process group does not deliver the signal to
// the parent (test) process. Without Setsid isolation, the child shares
// the parent's process group and the signal would kill the test.
func TestProcessIsolation_SignalDoesNotReachParent(t *testing.T) {
	t.Parallel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	defer signal.Stop(sigCh)

	var stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: `sh -c 'kill -INT -$$'`,
		Cwd:     t.TempDir(),
		Env:     os.Environ(),
		Stderr:  &stderr,
	})

	if err == nil {
		t.Log("child exited 0 after self-signal (unexpected but not a failure)")
	}

	select {
	case sig := <-sigCh:
		t.Fatalf("SIGINT leaked to parent process: %v (stderr=%q)", sig, stderr.String())
	case <-time.After(200 * time.Millisecond):
	}
}

// TestProcessIsolation_TTYAccessDoesNotBlock verifies that a child trying
// to read from /dev/tty does not block or suspend the parent. With Setsid,
// the child has no controlling terminal, so /dev/tty access fails immediately
// with ENXIO.
func TestProcessIsolation_TTYAccessDoesNotBlock(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, RunOptions{
			Command: `sh -c 'cat < /dev/tty'`,
			Cwd:     t.TempDir(),
			Env:     os.Environ(),
			Stderr:  &stderr,
		})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error from /dev/tty access in detached session, got nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("/dev/tty access blocked for >3s; session isolation may be broken (stderr=%q)", stderr.String())
	}
}

// TestProcessIsolation_ZshJobControlDoesNotSuspend simulates the exact
// scenario that caused the original bug: running zsh with job control
// enabled. Without session isolation, zsh tries to become the foreground
// process group leader on the controlling terminal, gets SIGTTIN, and
// suspends. Skips if zsh is not installed.
func TestProcessIsolation_ZshJobControlDoesNotSuspend(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not installed")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, RunOptions{
			Command: `zsh -i -c 'echo ok'`,
			Cwd:     t.TempDir(),
			Env:     os.Environ(),
			Stdout:  &stdout,
			Stderr:  &stderr,
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("zsh exited with error (may be expected): %v", err)
		}
		out := strings.TrimSpace(stdout.String())
		if !strings.Contains(out, "ok") {
			t.Errorf("expected 'ok' in stdout, got %q (stderr=%q)", out, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("zsh -i -c did not complete within 5s; likely suspended on TTY (stderr=%q)", stderr.String())
	}
}

// TestProcessIsolation_ChildProcessGroupKill verifies that when context
// is cancelled, the signal reaches the entire child process group (not
// just the direct child).
func TestProcessIsolation_ChildProcessGroupKill(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := Run(ctx, RunOptions{
		Command: `sh -c 'sleep 60 & wait'`,
		Cwd:     t.TempDir(),
		Env:     os.Environ(),
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	// Allow up to 4s: 500ms context timeout + 2s kill timeout + margin.
	if elapsed > 4*time.Second {
		t.Fatalf("cancellation took %v; expected prompt process group kill", elapsed)
	}
}
