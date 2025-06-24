package shell

import (
	"context"
	"testing"
	"time"
)

// Benchmark to measure CPU efficiency
func BenchmarkShellQuickCommands(b *testing.B) {
	shell := newPersistentShell(b.TempDir())

	b.ReportAllocs()

	for b.Loop() {
		_, _, err := shell.Exec(context.Background(), "echo test")
		exitCode := ExitCode(err)
		if err != nil || exitCode != 0 {
			b.Fatalf("Command failed: %v, exit code: %d", err, exitCode)
		}
	}
}

func TestTestTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	t.Cleanup(cancel)

	shell := newPersistentShell(t.TempDir())
	_, _, err := shell.Exec(ctx, "sleep 10")
	if status := ExitCode(err); status == 0 {
		t.Fatalf("Expected non-zero exit status, got %d", status)
	}
	if !IsInterrupt(err) {
		t.Fatalf("Expected command to be interrupted, but it was not")
	}
	if err == nil {
		t.Fatalf("Expected an error due to timeout, but got none")
	}
}

func TestTestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // immediately cancel the context

	shell := newPersistentShell(t.TempDir())
	_, _, err := shell.Exec(ctx, "sleep 10")
	if status := ExitCode(err); status == 0 {
		t.Fatalf("Expected non-zero exit status, got %d", status)
	}
	if !IsInterrupt(err) {
		t.Fatalf("Expected command to be interrupted, but it was not")
	}
	if err == nil {
		t.Fatalf("Expected an error due to cancel, but got none")
	}
}

func TestRunCommandError(t *testing.T) {
	shell := newPersistentShell(t.TempDir())
	_, _, err := shell.Exec(t.Context(), "nopenopenope")
	if status := ExitCode(err); status == 0 {
		t.Fatalf("Expected non-zero exit status, got %d", status)
	}
	if IsInterrupt(err) {
		t.Fatalf("Expected command to not be interrupted, but it was")
	}
	if err == nil {
		t.Fatalf("Expected an error, got nil")
	}
}

func TestRunContinuity(t *testing.T) {
	shell := newPersistentShell(t.TempDir())
	shell.Exec(t.Context(), "export FOO=bar")
	dst := t.TempDir()
	shell.Exec(t.Context(), "cd "+dst)
	out, _, _ := shell.Exec(t.Context(), "echo $FOO ; pwd")
	expect := "bar\n" + dst + "\n"
	if out != expect {
		t.Fatalf("Expected output %q, got %q", expect, out)
	}
}
