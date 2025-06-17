package shell

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellPerformanceComparison(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shell-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	shell := GetPersistentShell(tmpDir)
	defer shell.Close()

	// Test quick command
	start := time.Now()
	stdout, stderr, exitCode, _, err := shell.Exec(context.Background(), "echo 'hello'", 0)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "hello")
	assert.Empty(t, stderr)

	t.Logf("Quick command took: %v", duration)
}

// Benchmark CPU usage during polling
func BenchmarkShellPolling(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "shell-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	shell := GetPersistentShell(tmpDir)
	defer shell.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Use a short sleep to measure polling overhead
		_, _, exitCode, _, err := shell.Exec(context.Background(), "sleep 0.02", 500)
		if err != nil || exitCode != 0 {
			b.Fatalf("Command failed: %v, exit code: %d", err, exitCode)
		}
	}
}
