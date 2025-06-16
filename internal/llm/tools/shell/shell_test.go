package shell

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellPerformanceImprovement(t *testing.T) {
	// Create a temporary directory for the shell
	tmpDir, err := os.MkdirTemp("", "shell-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	shell := GetPersistentShell(tmpDir)
	defer shell.Close()

	// Test that quick commands complete fast
	start := time.Now()
	stdout, stderr, exitCode, _, err := shell.Exec(context.Background(), "echo 'hello world'", 0)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "hello world")
	assert.Empty(t, stderr)

	// Quick commands should complete very fast with our exponential backoff
	assert.Less(t, duration, 50*time.Millisecond, "Quick command should complete fast with exponential backoff")
}

// Benchmark to measure CPU efficiency
func BenchmarkShellQuickCommands(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "shell-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	shell := GetPersistentShell(tmpDir)
	defer shell.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, exitCode, _, err := shell.Exec(context.Background(), "echo test", 0)
		if err != nil || exitCode != 0 {
			b.Fatalf("Command failed: %v, exit code: %d", err, exitCode)
		}
	}
}
