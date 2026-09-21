package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAcquireLock_AcquiresWhenFree(t *testing.T) {
	t.Parallel()
	l, err := AcquireLock(t.TempDir(), "sess-1")
	require.NoError(t, err)
	require.NotNil(t, l)
	l.Release()
}

func TestAcquireLock_ContendsOnSameSession(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	l, err := AcquireLock(dataDir, "sess-1")
	require.NoError(t, err)
	t.Cleanup(l.Release)

	_, err = AcquireLock(dataDir, "sess-1")
	require.ErrorIs(t, err, ErrSessionLocked)
}

func TestAcquireLock_DifferentSessionsDoNotContend(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	l1, err := AcquireLock(dataDir, "sess-1")
	require.NoError(t, err)
	t.Cleanup(l1.Release)

	l2, err := AcquireLock(dataDir, "sess-2")
	require.NoError(t, err)
	t.Cleanup(l2.Release)
}

func TestAcquireLock_ReacquireAfterRelease(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	l, err := AcquireLock(dataDir, "sess-1")
	require.NoError(t, err)
	l.Release()

	l2, err := AcquireLock(dataDir, "sess-1")
	require.NoError(t, err)
	t.Cleanup(l2.Release)
}

func TestLock_ReleaseOnNilIsSafe(t *testing.T) {
	t.Parallel()
	var l *Lock
	l.Release()
}
