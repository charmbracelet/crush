package dirid

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOf checks that two spellings of a directory share an ID and distinct
// directories do not.
func TestOf(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	require.NoError(t, os.Mkdir(a, 0o755))
	require.NoError(t, os.Mkdir(b, 0o755))
	if err := os.Symlink(a, filepath.Join(root, "link")); err != nil {
		t.Skipf("Cannot create symlink: %v", err)
	}

	ida, _ := Of(a)
	idb, _ := Of(b)
	idl, ok := Of(filepath.Join(root, "link"))
	require.True(t, ok)
	require.True(t, ida.Same(idl))
	require.False(t, ida.Same(idb))
	require.False(t, ID{}.Same(ID{}))
}

// TestValidFileID checks that the all-zero and all-ones IDs some
// filesystems (e.g. ReFS) report are never treated as an identity.
func TestValidFileID(t *testing.T) {
	t.Parallel()
	require.False(t, validFileIndex(0))
	require.False(t, validFileIndex(0xFFFFFFFFFFFFFFFF))
	require.True(t, validFileIndex(1))
	var id [16]byte
	require.False(t, validFileID(id))
	for i := range id {
		id[i] = 0xff
	}
	require.False(t, validFileID(id))
	require.True(t, validFileID([16]byte{1}))
}
