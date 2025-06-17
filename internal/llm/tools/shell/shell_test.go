package shell

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Benchmark to measure CPU efficiency
func BenchmarkShellQuickCommands(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "shell-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	shell := GetPersistentShell(tmpDir)
	defer shell.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _, exitCode, _, err := shell.Exec(context.Background(), "echo test", 0)
		if err != nil || exitCode != 0 {
			b.Fatalf("Command failed: %v, exit code: %d", err, exitCode)
		}
	}
}