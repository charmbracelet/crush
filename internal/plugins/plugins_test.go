package plugins

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseSource(t *testing.T) {
	cases := []struct {
		spec   string
		source string
		ref    string
		Dir    string
		err    bool
	}{
		{spec: "example/crush-plugins", source: "example/crush-plugins", Dir: "example__crush-plugins"},
		{spec: "Example/Crush-Plugins@v1.2.3", source: "Example/Crush-Plugins", ref: "v1.2.3", Dir: "example__crush-plugins"},
		{spec: "a/b@release/2", source: "a/b", ref: "release/2", Dir: "a__b"},
		{spec: "  a/b  ", source: "a/b", Dir: "a__b"},
		{spec: "justaname", err: true},
		{spec: "a/b/../c", err: true},
		{spec: "a//b", err: true},
		{spec: "a/b@..", err: true},
		{spec: "a/b@v1?q=1", err: true},
	}
	for _, tc := range cases {
		src, ref, err := ParseSource(tc.spec)
		if tc.err {
			require.Error(t, err, tc.spec)
			continue
		}
		require.NoError(t, err, tc.spec)
		require.Equal(t, tc.source, src.String(), tc.spec)
		require.Equal(t, tc.ref, ref, tc.spec)
		require.Equal(t, tc.Dir, src.DirName(), tc.spec)
	}
}

func TestValidRef(t *testing.T) {
	for _, ref := range []string{"main", "v1.2.3", "release/2", "feature/x-1_2", "0123456789abcdef", "-dash"} {
		require.True(t, ValidRef(ref), ref)
	}
	for _, ref := range []string{"", "..", "a/..", "a//b", "a?b", "a#b", "a b", `a\b`} {
		require.False(t, ValidRef(ref), ref)
	}
}

func TestRootsScopeSelection(t *testing.T) {
	global := filepath.Join("tmp", "config", "crush", "plugins")
	cwd := filepath.Join("tmp", "project")

	require.Equal(t, []ScopedRoot{
		{Scope: ScopeGlobal, Path: global},
		{Scope: ScopeProject, Path: filepath.Join(cwd, ".crush", "plugins")},
	}, Roots(global, cwd, false, false), "no scope flag means both")

	require.Equal(t, []ScopedRoot{{Scope: ScopeGlobal, Path: global}}, Roots(global, cwd, true, false))
	require.Equal(t, []ScopedRoot{{Scope: ScopeProject, Path: filepath.Join(cwd, ".crush", "plugins")}}, Roots(global, cwd, false, true))
}

func TestManifestRoundTripAndValidation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "example__repo")
	m := Manifest{
		Version:     ManifestVersion,
		Source:      "example/repo",
		Ref:         "main",
		Commit:      commitOne,
		CommitURL:   "https://github.com/example/repo/commit/" + commitOne,
		InstalledAt: time.Now().UTC().Truncate(time.Second),
		UpdatedAt:   time.Now().UTC().Truncate(time.Second),
		Files:       []File{{Path: "one.sh", SHA256: "abc", Size: 3}},
	}
	require.NoError(t, m.Save(dir))

	got, err := LoadManifest(dir)
	require.NoError(t, err)
	require.Equal(t, m.Source, got.Source)
	require.Equal(t, m.Commit, got.Commit)
	require.Equal(t, m.Files, got.Files)
	require.Equal(t, 8, len(got.ShortCommit()))
	require.Equal(t, filepath.Join(dir, ManifestName), ManifestPath(dir))

	info, err := os.Stat(ManifestPath(dir))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	// A missing lock is not an error a command should die on.
	_, err = LoadManifest(filepath.Join(filepath.Dir(dir), "nothing"))
	require.ErrorIs(t, err, ErrNoManifest)

	// Half-written or hand-edited garbage is reported, never trusted.
	bad := filepath.Join(t.TempDir(), "bad__repo")
	require.NoError(t, os.MkdirAll(bad, dirPerm))
	require.NoError(t, os.WriteFile(filepath.Join(bad, ManifestName), []byte("{not json"), manifestPerm))
	_, err = LoadManifest(bad)
	require.ErrorContains(t, err, "failed to parse plugin manifest")

	require.NoError(t, os.WriteFile(filepath.Join(bad, ManifestName), []byte(`{"version":99,"source":"a/b","commit":"x"}`), manifestPerm))
	_, err = LoadManifest(bad)
	require.ErrorContains(t, err, "version 99")

	require.NoError(t, os.WriteFile(filepath.Join(bad, ManifestName), []byte(`{"version":1,"source":"a/b"}`), manifestPerm))
	_, err = LoadManifest(bad)
	require.ErrorContains(t, err, "missing its source or commit")
}

func TestManifestDiff(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), scriptPerm))
	}
	write("one.sh", "provider add one")
	write("two.sh", "provider add two")
	write("edited.sh", "provider add edited")
	write(".hidden.sh", "provider add hidden")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), dirPerm))
	write("nested/deep.sh", "provider add deep")

	files, err := HashDir(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"edited.sh", "one.sh", "two.sh"}, []string{files[0].Path, files[1].Path, files[2].Path}, "hidden and nested files are not plugins")
	require.Equal(t, len("provider add one"), int(files[1].Size))

	m := Manifest{Source: "a/b", Commit: commitOne, Files: []File{
		{Path: "one.sh", SHA256: files[1].SHA256},
		{Path: "gone.sh", SHA256: "whatever"},
		{Path: "edited.sh", SHA256: "stale"},
	}}

	diff, err := m.Diff(dir)
	require.NoError(t, err)
	require.Equal(t, Diff{
		Added:   []string{"two.sh"},
		Removed: []string{"gone.sh"},
		Changed: []string{"edited.sh"},
	}, diff)
	require.False(t, diff.Empty())
	require.Equal(t, []string{"edited.sh", "gone.sh", "two.sh"}, diff.Names())

	clean := Manifest{Source: "a/b", Commit: commitOne, Files: files}
	diff, err = clean.Diff(dir)
	require.NoError(t, err)
	require.True(t, diff.Empty())
}

func TestDigestMatchesFileHash(t *testing.T) {
	body := []byte("provider add one")
	path := filepath.Join(t.TempDir(), "one.sh")
	require.NoError(t, os.WriteFile(path, body, scriptPerm))

	onDisk, err := HashFile(path)
	require.NoError(t, err)
	fromBytes := Digest(body)
	require.Equal(t, fromBytes.SHA256, onDisk.SHA256)
	require.Equal(t, int64(len(body)), onDisk.Size)
}
