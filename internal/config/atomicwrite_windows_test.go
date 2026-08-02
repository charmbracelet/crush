//go:build windows

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// Regression test for the windows-latest CI flake where
// TestConfigStore_SetConfigFields_concurrentInProcess failed with
// "rename ...crush.json.<rand>.tmp ...crush.json: Access is denied":
// renaming over a destination while another handle is open on it
// without sharing must be retried until the handle closes, not
// surfaced as an error.
func TestAtomicWriteFile_RetriesWhileDestinationHandleOpen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	require.NoError(t, atomicWriteFile(path, []byte(`{"v":1}`), 0o600))

	// Hold the destination open with no sharing flags so the rename
	// fails with ERROR_ACCESS_DENIED/ERROR_SHARING_VIOLATION until the
	// handle closes, mimicking an antivirus scan or exclusive reader.
	p, err := windows.UTF16PtrFromString(path)
	require.NoError(t, err)
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	require.NoError(t, err)
	go func() {
		time.Sleep(200 * time.Millisecond)
		windows.CloseHandle(h)
	}()

	require.NoError(t, atomicWriteFile(path, []byte(`{"v":2}`), 0o600))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{"v":2}`, string(data))
}
