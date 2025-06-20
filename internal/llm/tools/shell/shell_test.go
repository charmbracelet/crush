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
		_, _, exitCode, _, err := shell.Exec(context.Background(), "echo test")
		if err != nil || exitCode != 0 {
			b.Fatalf("Command failed: %v, exit code: %d", err, exitCode)
		}
	}
}

func TestTestTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	t.Cleanup(cancel)

	shell := newPersistentShell(t.TempDir())
	_, _, status, interrupted, err := shell.Exec(ctx, "sleep 10")
	if status == 0 {
		t.Fatalf("Expected non-zero exit status, got %d", status)
	}
	if !interrupted {
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
	_, _, status, interrupted, err := shell.Exec(ctx, "sleep 10")
	if status == 0 {
		t.Fatalf("Expected non-zero exit status, got %d", status)
	}
	if !interrupted {
		t.Fatalf("Expected command to be interrupted, but it was not")
	}
	if err == nil {
		t.Fatalf("Expected an error due to timeout, but got none")
	}
}
