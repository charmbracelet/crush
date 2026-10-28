package db

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAcquireDataDirLock_ContenderSeesOwnerInfo checks that a contender
// can read the owner details the holder records in the lock file. Two
// acquisitions in one process contend like two processes would, because
// each opens its own handle on the lock file.
func TestAcquireDataDirLock_ContenderSeesOwnerInfo(t *testing.T) {
	dataDir := t.TempDir()

	held, err := acquireDataDirLock(dataDir)
	require.NoError(t, err)
	t.Cleanup(held.release)

	_, err = acquireDataDirLock(dataDir)
	require.ErrorIs(t, err, ErrDataDirLocked)
	require.Contains(t, err.Error(), fmt.Sprintf("owner pid=%d ", os.Getpid()))
}
