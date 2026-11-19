package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutsideRoots(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "work", "project")

	// An empty root set turns enforcement off, which is the CLI and test
	// behavior.
	require.False(t, OutsideRoots(nil, filepath.Join(root, "f.go")))
	require.False(t, OutsideRoots(nil, "/etc/passwd"))

	// Inside the root, including the root itself.
	require.False(t, OutsideRoots([]string{root}, root))
	require.False(t, OutsideRoots([]string{root}, filepath.Join(root, "a", "b.go")))

	// Outside the root.
	parent := filepath.Dir(root)
	require.True(t, OutsideRoots([]string{root}, parent))
	require.True(t, OutsideRoots([]string{root}, filepath.Join(parent, "sibling", "f.go")))

	// A sibling whose name starts with the root's own name must not pass the
	// prefix trap: component-wise comparison, not string prefix (the spec asks
	// for exactly this, session-setup.md:452).
	require.True(t, OutsideRoots([]string{root}, root+"-evil/f.go"))

	// Any one matching root is enough.
	require.False(t, OutsideRoots([]string{root, parent}, filepath.Join(parent, "x")))
}

// fakeClientFS is a client file system backed by maps, so a test can prove the
// tools route through it instead of the disk.
type fakeClientFS struct {
	reads   map[string]string
	writes  map[string]string
	readErr error
}

func (f *fakeClientFS) ReadTextFile(_ context.Context, path string) (string, error) {
	if f.readErr != nil {
		return "", f.readErr
	}
	text, ok := f.reads[path]
	if !ok {
		return "", os.ErrNotExist
	}
	return text, nil
}

func (f *fakeClientFS) WriteTextFile(_ context.Context, path, content string) error {
	f.writes[path] = content
	return nil
}

func TestReadWriteFileTextDelegatesToClientFS(t *testing.T) {
	// This test mutates the package-global client fs, so it cannot run in
	// parallel with anything that reads or writes files.
	fake := &fakeClientFS{
		reads:  map[string]string{"/virt/file.txt": "from client"},
		writes: map[string]string{},
	}
	SetClientFS(fake)
	t.Cleanup(func() { SetClientFS(nil) })

	got, err := ReadFileText(context.Background(), "/virt/file.txt")
	require.NoError(t, err)
	require.Equal(t, "from client", got)

	require.NoError(t, WriteFileText(context.Background(), "/virt/out.txt", "hello"))
	require.Equal(t, "hello", fake.writes["/virt/out.txt"])

	// A read the client cannot satisfy surfaces its error rather than silently
	// falling back to the disk: the client owns the file set.
	fake.readErr = errors.New("gone")
	_, err = ReadFileText(context.Background(), "/virt/file.txt")
	require.Error(t, err)
}

func TestReadWriteFileTextUsesDiskWithoutClientFS(t *testing.T) {
	SetClientFS(nil)
	path := filepath.Join(t.TempDir(), "f.txt")
	require.NoError(t, WriteFileText(context.Background(), path, "on disk"))
	got, err := ReadFileText(context.Background(), path)
	require.NoError(t, err)
	require.Equal(t, "on disk", got)
}
