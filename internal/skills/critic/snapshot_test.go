package critic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotStore_CaptureAndRollback(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "foo.txt")
	require.NoError(t, os.WriteFile(p, []byte("before"), 0o644))

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p}))

	require.NoError(t, os.WriteFile(p, []byte("after"), 0o644))
	require.NoError(t, ss.Rollback())

	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "before", string(b))
}

func TestSnapshotStore_PermissionPreservation(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "script.sh")
	require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh"), 0o755))

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p}))

	require.NoError(t, os.WriteFile(p, []byte("#!/bin/bash"), 0o644))
	require.NoError(t, ss.Rollback())

	info, err := os.Stat(p)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestSnapshotStore_DeletedFileRestored(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "bar.txt")
	require.NoError(t, os.WriteFile(p, []byte("content"), 0o644))

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p}))

	require.NoError(t, os.Remove(p))
	require.NoError(t, ss.Rollback())

	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "content", string(b))
}

func TestSnapshotStore_NewFileRemovedOnRollback(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "new.txt")

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p}))

	require.NoError(t, os.WriteFile(p, []byte("new"), 0o644))
	require.NoError(t, ss.Rollback())

	_, err := os.Stat(p)
	require.True(t, os.IsNotExist(err))
}

func TestSnapshotStore_Changed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p1 := filepath.Join(tmp, "a.txt")
	p2 := filepath.Join(tmp, "b.txt")
	require.NoError(t, os.WriteFile(p1, []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(p2, []byte("b"), 0o644))

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p1, p2}))

	require.NoError(t, os.WriteFile(p1, []byte("changed"), 0o644))

	changed, after, err := ss.Changed()
	require.NoError(t, err)
	require.Len(t, changed, 1)
	require.Equal(t, p1, changed[0])
	require.Equal(t, "changed", string(after[p1]))
}

func TestSnapshotStore_Clear(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "x.txt")
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))

	ss := NewSnapshotStore()
	require.NoError(t, ss.Capture([]string{p}))
	ss.Clear()
	require.Empty(t, ss.Paths())
}
