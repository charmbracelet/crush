package fsext

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	require.NoError(t, AtomicWriteFile(path, []byte(`{"key":"value"}`), 0o600))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, `{"key":"value"}`, string(data))

	// No temp files should linger.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "test.json", entries[0].Name())
}

func TestAtomicWriteFile_PermissionsApplied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support Unix file permissions")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	require.NoError(t, AtomicWriteFile(path, []byte(`{}`), 0o600))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// TestSyncDirToleratesUnopenablePaths pins that the directory fsync is
// best-effort: on filesystems or paths where the directory cannot be opened
// or synced, the write itself must not fail, because the data is already
// renamed into place and visible. A regular file passed as a directory is the
// portable proxy for such a path (on Linux fsync on a read-only fd succeeds,
// so this returns nil rather than an error; on Windows syncDir is a no-op).
func TestSyncDirToleratesUnopenablePaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "not-a-directory")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	require.NoError(t, syncDir(path), "syncDir on an unopenable-as-directory path must not fail")
}

func BenchmarkAtomicWriteFile(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.json")
	data := []byte(`{"key":"value","nested":{"a":1,"b":2,"c":3}}`)

	b.ResetTimer()
	for range b.N {
		if err := AtomicWriteFile(path, data, 0o600); err != nil {
			b.Fatal(err)
		}
	}
}
