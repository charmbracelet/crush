package log

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// The cleanup a caller hands RecoverPanic runs even when the panic log
// cannot be written: the TUI's subscription passes program.Quit there, and
// skipping it leaves a TUI that no longer receives any event.
func TestRecoverPanicRunsCleanupWhenLogFileFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the working directory cannot be removed on Windows")
	}

	// A working directory that no longer exists, so os.Create fails
	// whoever runs the test.
	dir := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, os.Mkdir(dir, 0o755))
	t.Chdir(dir)
	require.NoError(t, os.Remove(dir))

	cleaned := false
	func() {
		defer RecoverPanic("test", func() { cleaned = true })
		panic("boom")
	}()

	require.True(t, cleaned, "cleanup was skipped")
}
