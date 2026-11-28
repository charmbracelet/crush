package plugins

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTarGzStripsTheTopDirectory(t *testing.T) {
	dir := t.TempDir()
	written, err := ExtractTarGz(bytes.NewReader(tarGz(t, "crush-plugins-abc123", map[string]string{
		"one.sh":        "provider add one",
		"two.sh":        "provider add two",
		"lib/helper.sh": "vendored helper",
	})), dir)
	require.NoError(t, err)
	require.Equal(t, int64(len("provider add one")+len("provider add two")+len("vendored helper")), written)

	require.FileExists(t, filepath.Join(dir, "one.sh"))
	require.FileExists(t, filepath.Join(dir, "two.sh"))

	// Nested content is unpacked but is not a plugin, which is what keeps the
	// repository root as the whole contract.
	names, err := PluginFiles(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"one.sh", "two.sh"}, names)
	require.FileExists(t, filepath.Join(dir, "lib", "helper.sh"))
}

func TestExtractTarGzIgnoresHostileEntries(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()

	var raw bytes.Buffer
	zw := gzip.NewWriter(&raw)
	tw := tar.NewWriter(zw)
	add := func(hdr *tar.Header, body string) {
		tw.WriteHeader(hdr)
		tw.Write([]byte(body))
	} // An archive can name files that would land outside the staging directory,
	// or create links that point wherever its author likes.
	add(&tar.Header{Name: "../escaped.sh", Typeflag: tar.TypeReg, Size: 3, Mode: 0o644}, "bad")
	add(&tar.Header{Name: filepath.Base(dir) + "/../escaped2.sh", Typeflag: tar.TypeReg, Size: 3, Mode: 0o644}, "bad")
	add(&tar.Header{Name: "/etc/passwd", Typeflag: tar.TypeReg, Size: 3, Mode: 0o644}, "bad")
	add(&tar.Header{Name: "top/link.sh", Typeflag: tar.TypeSymlink, Linkname: filepath.Join(outside, "victim.sh")}, "")
	add(&tar.Header{Name: "top/good.sh", Typeflag: tar.TypeReg, Size: 4, Mode: 0o644}, "fine")
	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())

	_, err := ExtractTarGz(bytes.NewReader(raw.Bytes()), dir)
	require.NoError(t, err)

	require.NoFileExists(t, filepath.Join(filepath.Dir(dir), "escaped.sh"))
	require.NoFileExists(t, filepath.Join(filepath.Dir(dir), "escaped2.sh"))
	require.NoFileExists(t, filepath.Join(outside, "victim.sh"))

	names, err := PluginFiles(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"good.sh"}, names, "only the plain regular file should survive")

	info, err := os.Lstat(filepath.Join(dir, "link.sh"))
	require.Error(t, err, "a link from the archive must not be created")
	if err == nil {
		require.False(t, info.Mode()&os.ModeSymlink != 0)
	}
}

func TestExtractTarGzRejectsAnOversizedFile(t *testing.T) {
	dir := t.TempDir()
	var raw bytes.Buffer
	zw := gzip.NewWriter(&raw)
	tw := tar.NewWriter(zw)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "top/huge.sh",
		Typeflag: tar.TypeReg,
		Size:     maxEntrySize + 1,
		Mode:     0o644,
	}))
	_, err := tw.Write(make([]byte, maxEntrySize+1))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())

	_, err = ExtractTarGz(bytes.NewReader(raw.Bytes()), dir)
	require.ErrorContains(t, err, "over the")
	require.NoDirExists(t, filepath.Join(dir, "huge.sh"))
}

func TestExtractTarGzRejectsGarbage(t *testing.T) {
	_, err := ExtractTarGz(bytes.NewReader([]byte("not a gzip stream")), t.TempDir())
	require.ErrorContains(t, err, "failed to read the archive")
}

func TestArchivePathDropsTheTopDirectory(t *testing.T) {
	cases := []struct {
		name string
		want string
		ok   bool
	}{
		{"repo-abc/one.sh", "one.sh", true},
		{"repo-abc/sub/one.sh", "sub/one.sh", true},
		{"one.sh", "", false},
		{"repo-abc/", "", false},
		{"/repo-abc/one.sh", "one.sh", true},
		// A leading .. cannot survive the clean, so it collapses into the
		// archive's own top directory instead of escaping it.
		{"../repo-abc/one.sh", "one.sh", true},
		{"../../one.sh", "", false},
	}
	for _, tc := range cases {
		got, ok := archivePath(tc.name)
		require.Equal(t, tc.want, got, tc.name)
		require.Equal(t, tc.ok, ok, tc.name)
	}
}
