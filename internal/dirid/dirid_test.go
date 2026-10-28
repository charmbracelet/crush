package dirid

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidFileIndex(t *testing.T) {
	t.Parallel()
	require.False(t, validFileIndex(0))
	require.False(t, validFileIndex(0xFFFFFFFFFFFFFFFF))
	require.True(t, validFileIndex(1))
	require.True(t, validFileIndex(0x0001000000001234))
}

func TestValidFileID(t *testing.T) {
	t.Parallel()
	var id [16]byte
	require.False(t, validFileID(id))
	for i := range id {
		id[i] = 0xff
	}
	require.False(t, validFileID(id))
	id[15] = 0
	require.True(t, validFileID(id))
	require.True(t, validFileID([16]byte{1}))
}

func TestZeroIDNeverSame(t *testing.T) {
	t.Parallel()
	var zero ID
	require.False(t, zero.Same(zero))
	id, ok := Of(t.TempDir())
	require.True(t, ok)
	require.False(t, zero.Same(id))
	require.False(t, id.Same(zero))
}

func TestOf_MissingPath(t *testing.T) {
	t.Parallel()
	_, ok := Of(filepath.Join(t.TempDir(), "missing"))
	require.False(t, ok)
}

func TestOf_DistinguishesAndUnifies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	require.NoError(t, os.Mkdir(a, 0o755))
	require.NoError(t, os.Mkdir(b, 0o755))

	ida, ok := Of(a)
	require.True(t, ok)
	idb, ok := Of(b)
	require.True(t, ok)
	ida2, ok := Of(a + string(filepath.Separator) + ".")
	require.True(t, ok)

	require.True(t, ida.Same(ida2))
	require.False(t, ida.Same(idb))

	link := filepath.Join(root, "link")
	if err := os.Symlink(a, link); err != nil {
		t.Logf("Skipping symlink check: %v", err)
		return
	}
	idl, ok := Of(link)
	require.True(t, ok)
	require.True(t, ida.Same(idl))
}

func TestOf_CaseVariant(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "Proj")
	require.NoError(t, os.Mkdir(dir, 0o755))
	lower := filepath.Join(root, "proj")
	if _, err := os.Stat(lower); err != nil {
		t.Skip("Filesystem is case-sensitive")
	}
	id, ok := Of(dir)
	require.True(t, ok)
	idl, ok := Of(lower)
	require.True(t, ok)
	require.True(t, id.Same(idl))
}
