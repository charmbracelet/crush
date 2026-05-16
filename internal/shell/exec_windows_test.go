//go:build windows

package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestRun_CtxCancel_KillsProcessTree verifies that cancelling a running
// command terminates the entire process tree, not just the direct child.
//
// Structure:
//  1. Run() launches powershell.exe via the standard exec handler chain.
//     startedCmd assigns powershell.exe to a Windows Job Object.
//  2. powershell.exe forks a grandchild (ping.exe) via Start-Process.
//     The grandchild inherits the job association automatically.
//  3. The grandchild writes its PID to a file, then runs for 10 minutes.
//  4. The test cancels the context.
//  5. TerminateJobObject kills both powershell.exe and ping.exe.
//  6. The test confirms ping.exe is gone via OpenProcess.
//
// Without the Job Object fix, killing powershell.exe leaves ping.exe
// orphaned and running until its 10-minute timeout expires.
func TestRun_CtxCancel_KillsProcessTree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")

	// PowerShell script: fork a grandchild (ping 600s), write its PID to
	// pidfile, then wait on it. When we cancel, TerminateJobObject kills
	// both the PowerShell parent and the ping grandchild.
	psContent := fmt.Sprintf(`
$process = Start-Process -NoNewWindow -PassThru powershell -ArgumentList '-Command "ping -n 600 127.0.0.1"'
$process.Id | Out-File -FilePath '%s' -Encoding ASCII
Wait-Process -Id $process.Id
`, pidfile)

	psScript := filepath.Join(dir, "spawn.ps1")
	if err := os.WriteFile(psScript, []byte(psContent), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, RunOptions{
			Command: fmt.Sprintf(`powershell -ExecutionPolicy Bypass -File "%s"`, psScript),
			Cwd:     dir,
			Env:     os.Environ(),
		})
	}()

	// Wait for the grandchild to write its PID.
	grandchildPid := waitForPIDFileWindows(t, pidfile, 10*time.Second)

	// Cancel the parent — this should kill everything in the Job Object.
	cancel()

	select {
	case err := <-done:
		if err != nil && !IsInterrupt(err) {
			t.Fatalf("Run returned unexpected error: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return within 30s of ctx cancel")
	}

	// Verify the grandchild process is dead. On Windows, OpenProcess
	// against a non-existent PID returns ERROR_INVALID_PARAMETER.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h, err := windows.OpenProcess(
			windows.PROCESS_QUERY_INFORMATION, false, uint32(grandchildPid),
		)
		if err != nil {
			return // process is gone — success
		}
		windows.CloseHandle(h)
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("grandchild pid %d still alive after cancel; the Job Object fix did not reach it", grandchildPid)
}

// TestRun_CtxCancel_ReturnsContextErr is identical in intent to the
// Unix version — it verifies that a cancelled command surfaces
// ctx.Err() rather than a synthetic exit code.
func TestRun_CtxCancel_ReturnsContextErr(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, RunOptions{
			Command: "ping -n 600 127.0.0.1",
			Cwd:     t.TempDir(),
			Env:     os.Environ(),
		})
	}()

	// Give the ping a chance to start before cancelling.
	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return within 30s of ctx cancel")
	}
}

// waitForPIDFileWindows polls path until it contains a positive integer
// or the timeout expires. The PID-file dance mirrors exec_unix_test.go's
// waitForPIDFile, duplicated here to keep each build-tagged file self-
// contained.
func waitForPIDFileWindows(t *testing.T, path string, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			var pid int
			if _, err := fmt.Sscanf(string(data), "%d", &pid); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("pidfile %s never contained a valid pid", path)
	return 0
}
