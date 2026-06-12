//go:build windows

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteFile_RetriesWhileDestinationReaderOpen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	initial := []byte(`{"initial":true}`)
	updated := []byte(`{"updated":true}`)

	require.NoError(t, os.WriteFile(path, initial, 0o600))
	reader, err := os.Open(path)
	require.NoError(t, err)

	closeErr := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		closeErr <- reader.Close()
	}()

	require.NoError(t, atomicWriteFile(path, updated, 0o600))
	require.NoError(t, <-closeErr)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, updated, data)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "test.json", entries[0].Name())
}

func TestIsRetryableRenameError_Windows(t *testing.T) {
	t.Parallel()

	require.True(t, isRetryableRenameError(&os.LinkError{
		Op:  "rename",
		Old: "old",
		New: "new",
		Err: windows.ERROR_ACCESS_DENIED,
	}))
	require.True(t, isRetryableRenameError(&os.LinkError{
		Op:  "rename",
		Old: "old",
		New: "new",
		Err: windows.ERROR_SHARING_VIOLATION,
	}))
	require.False(t, isRetryableRenameError(&os.LinkError{
		Op:  "rename",
		Old: "old",
		New: "new",
		Err: windows.ERROR_FILE_NOT_FOUND,
	}))
}
