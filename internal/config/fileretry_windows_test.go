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

// A config read racing another writer's rename, or a scanner holding the
// file, must wait for the handle to close rather than fail the reload.
func TestReadFile_RetriesWhileHandleOpen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"v":1}`), 0o600))

	p, err := windows.UTF16PtrFromString(path)
	require.NoError(t, err)
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	require.NoError(t, err)

	_, err = os.ReadFile(path)
	require.Error(t, err, "an exclusive handle should block a plain read")

	go func() {
		time.Sleep(200 * time.Millisecond)
		windows.CloseHandle(h)
	}()

	data, err := readFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{"v":1}`, string(data))
}
