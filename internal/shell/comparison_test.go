package shell

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShellPerformanceComparison(t *testing.T) {
	shell := NewShell(&Options{WorkingDir: t.TempDir()})

	// Test quick command
	start := time.Now()
	stdout, stderr, err := shell.Exec(t.Context(), "echo 'hello'")
	exitCode := ExitCode(err)
	duration := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
	require.Contains(t, stdout, "hello")
	require.Empty(t, stderr)

	t.Logf("Quick command took: %v", duration)
}

// Benchmark CPU usage during polling
func BenchmarkShellPolling(b *testing.B) {
	shell := NewShell(&Options{WorkingDir: b.TempDir()})

	b.ReportAllocs()

	for b.Loop() {
		// Use a short sleep to measure polling overhead
		_, _, err := shell.Exec(b.Context(), "sleep 0.02")
		exitCode := ExitCode(err)
		if err != nil || exitCode != 0 {
			b.Fatalf("Command failed: %v, exit code: %d", err, exitCode)
		}
	}
}
