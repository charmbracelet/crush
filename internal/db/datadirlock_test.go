package db

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAcquireDataDirLock_ContenderSeesOwnerInfo guards that a contender can
// still read the holder's owner info, which a whole-file lock on Windows
// made unreadable.
func TestAcquireDataDirLock_ContenderSeesOwnerInfo(t *testing.T) {
	dataDir := t.TempDir()

	held, err := acquireDataDirLock(dataDir)
	require.NoError(t, err)
	t.Cleanup(held.release)

	_, err = acquireDataDirLock(dataDir)
	require.ErrorIs(t, err, ErrDataDirLocked)
	require.Contains(t, err.Error(), fmt.Sprintf("owner pid=%d ", os.Getpid()))
}
