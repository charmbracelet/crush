package shell

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellPerformanceComparison(t *testing.T) {
	shell := newPersistentShell(t.TempDir())

	// Test quick command
	start := time.Now()
	stdout, stderr, exitCode, _, err := shell.Exec(t.Context(), "echo 'hello'", 0)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "hello")
	assert.Empty(t, stderr)

	t.Logf("Quick command took: %v", duration)
}

// Benchmark CPU usage during polling
func BenchmarkShellPolling(b *testing.B) {
	shell := newPersistentShell(b.TempDir())

	b.ReportAllocs()

	for b.Loop() {
		// Use a short sleep to measure polling overhead
		_, _, exitCode, _, err := shell.Exec(b.Context(), "sleep 0.02", 0)
		if err != nil || exitCode != 0 {
			b.Fatalf("Command failed: %v, exit code: %d", err, exitCode)
		}
	}
}
