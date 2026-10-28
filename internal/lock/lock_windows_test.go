//go:build windows

package lock

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// lockWholeFile locks the whole of f exclusively, as older crush releases
// do, so a test can stand in for one of them.
func lockWholeFile(f *os.File) error {
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	return windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, math.MaxUint32, math.MaxUint32, new(windows.Overlapped))
}

// TestTryFile_ExcludesWholeFileLockers checks that the lock and a
// whole-file lock exclude each other in both directions, so an older and
// a newer crush release never hold the same lock file at once.
func TestTryFile_ExcludesWholeFileLockers(t *testing.T) {
	t.Parallel()

	t.Run("whole-file holder", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "test.lock")

		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
		require.NoError(t, err)
		t.Cleanup(func() { f.Close() })
		require.NoError(t, lockWholeFile(f))

		_, err = TryFile(path)
		require.ErrorIs(t, err, ErrContended)
	})

	t.Run("whole-file contender", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "test.lock")

		release, err := TryFile(path)
		require.NoError(t, err)
		t.Cleanup(release)

		f, err := os.OpenFile(path, os.O_RDWR, 0)
		require.NoError(t, err)
		t.Cleanup(func() { f.Close() })
		require.ErrorIs(t, lockWholeFile(f), windows.ERROR_LOCK_VIOLATION)
	})
}
