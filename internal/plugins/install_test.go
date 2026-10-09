package plugins

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	oneScript = `provider add one --type openai-compat --base-url "https://one.example/v1" --api-key k`
	twoScript = `provider add two --type openai-compat --base-url "https://two.example/v1" --api-key k`

	commitOne = "1111111111111111111111111111111111111111"
	commitTwo = "2222222222222222222222222222222222222222"
	commitV2  = "3333333333333333333333333333333333333333"

	testRepo = "example/crush-plugins"
)

func install(t *testing.T, root string, f *fakeGitHub, spec string) Result {
	t.Helper()
	src, ref, err := ParseSource(spec)
	require.NoError(t, err)
	res, err := Install(context.Background(), Options{Root: root, Source: src, Ref: ref, GitHub: f.client()})
	require.NoError(t, err)
	return res
}

func TestInstallWritesPluginsAndLock(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{
		"one.sh":        oneScript,
		"two.sh":        twoScript,
		"README.md":     "not a plugin",
		"lib/helper.sh": "vendored, not a plugin",
	})

	res := install(t, root, f, testRepo)
	dir := filepath.Join(root, "example__crush-plugins")
	require.Equal(t, dir, res.Dir)
	require.Empty(t, res.Previous, "a first install replaces nothing")

	require.FileExists(t, filepath.Join(dir, "one.sh"))
	require.FileExists(t, filepath.Join(dir, "two.sh"))
	require.NoFileExists(t, filepath.Join(dir, "README.md"))
	require.NoDirExists(t, filepath.Join(dir, "lib"))

	body, err := os.ReadFile(filepath.Join(dir, "one.sh"))
	require.NoError(t, err)
	require.Equal(t, oneScript, string(body))

	info, err := os.Stat(filepath.Join(dir, "one.sh"))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "a plugin is a script")
	}

	require.Equal(t, testRepo, res.Manifest.Source)
	require.Equal(t, DefaultRef, res.Manifest.Ref)
	require.Equal(t, commitOne, res.Manifest.Commit)
	require.Equal(t, "https://github.com/"+testRepo+"/commit/"+commitOne, res.Manifest.CommitURL)
	require.Len(t, res.Manifest.Files, 2)
	require.Equal(t, []string{"one.sh", "two.sh"}, res.Manifest.FileNames())
	require.NotEmpty(t, res.Manifest.Files[0].SHA256)
	require.False(t, res.Manifest.InstalledAt.IsZero())

	// The record describes what landed, so a fresh install is never modified.
	diff, err := res.Manifest.Diff(dir)
	require.NoError(t, err)
	require.True(t, diff.Empty(), "the lock should match the files just written: %+v", diff)

	// Nothing from the download is left behind.
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 1, "only the plugin directory should remain in the plugins root")

	// Installing the same commit again changes nothing and costs one lookup.
	before := len(f.seen)
	again := install(t, root, f, testRepo)
	require.True(t, again.UpToDate)
	require.Len(t, f.seen, before+1)
}

func TestInstallFollowsPinnedRef(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, "v1.2.3", commitOne, map[string]string{"one.sh": oneScript})
	f.publish(testRepo, DefaultRef, commitTwo, map[string]string{"one.sh": twoScript})

	res := install(t, root, f, testRepo+"@v1.2.3")
	require.Equal(t, "v1.2.3", res.Manifest.Ref)
	require.Equal(t, commitOne, res.Manifest.Commit)

	// An update follows the recorded ref, so a moving default branch cannot
	// drag a pinned install along with it.
	src, _, err := ParseSource(testRepo)
	require.NoError(t, err)
	updated, err := Update(context.Background(), UpdateOptions{Root: root, Source: src, GitHub: f.client()})
	require.NoError(t, err)
	require.True(t, updated.UpToDate)
	require.Equal(t, commitOne, updated.Manifest.Commit)
}

func TestInstallWithoutPluginsFails(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"README.md": "docs only"})

	src, _, err := ParseSource(testRepo)
	require.NoError(t, err)
	_, err = Install(context.Background(), Options{Root: root, Source: src, GitHub: f.client()})
	require.ErrorContains(t, err, "no plugins found")
	require.NoDirExists(t, Dir(root, src))
}

func TestInstallRejectsDirectoryOwnedByAnotherRepo(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	// a__b/c and a/b__c both map to the directory a__b__c.
	f.publish("a__b/c", DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	f.publish("a/b__c", DefaultRef, commitTwo, map[string]string{"two.sh": twoScript})

	install(t, root, f, "a__b/c")
	require.FileExists(t, filepath.Join(root, "a__b__c", "one.sh"))

	src, _, err := ParseSource("a/b__c")
	require.NoError(t, err)
	_, err = Install(context.Background(), Options{Root: root, Source: src, GitHub: f.client()})
	require.ErrorContains(t, err, "already holds plugins from")
	require.NoFileExists(t, filepath.Join(root, "a__b__c", "two.sh"))
}

func TestInstallReportsUnreadableRepo(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.status["/repos/"+testRepo+"/commits/"+DefaultRef] = 404

	_, err := installErr(t, root, f, testRepo)
	require.ErrorContains(t, err, "not found. If it is private, export GITHUB_TOKEN")
}

func installErr(t *testing.T, root string, f *fakeGitHub, spec string) (Result, error) {
	t.Helper()
	src, ref, err := ParseSource(spec)
	require.NoError(t, err)
	return Install(context.Background(), Options{Root: root, Source: src, Ref: ref, GitHub: f.client()})
}

func TestDiscoverAndFind(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	f.publish("other/plugins", DefaultRef, commitTwo, map[string]string{"two.sh": twoScript})

	install(t, global, f, testRepo)
	install(t, project, f, "other/plugins")
	// A hand-made subdirectory with no lock is not a managed install.
	require.NoError(t, os.MkdirAll(filepath.Join(global, "by-hand"), dirPerm))
	require.NoError(t, os.WriteFile(filepath.Join(global, "by-hand", "local.sh"), []byte(oneScript), scriptPerm))
	// Neither is a damaged lock, but it must be reported rather than skipped.
	corrupt := filepath.Join(global, "broken__repo")
	require.NoError(t, os.MkdirAll(corrupt, dirPerm))
	require.NoError(t, os.WriteFile(filepath.Join(corrupt, ManifestName), []byte("{"), manifestPerm))

	roots := []ScopedRoot{{Scope: ScopeGlobal, Path: global}, {Scope: ScopeProject, Path: project}}
	all, err := Discover(roots)
	require.NoError(t, err)
	require.Len(t, all, 3)

	var managed, broken int
	for _, in := range all {
		if in.Err != nil {
			broken++
			require.Contains(t, in.Dir, "broken__repo")
			continue
		}
		managed++
		require.False(t, in.Modified)
	}
	require.Equal(t, 2, managed)
	require.Equal(t, 1, broken)

	src, _, err := ParseSource(strings.ToUpper(testRepo))
	require.NoError(t, err)
	matches, err := Find(roots, src)
	require.NoError(t, err)
	require.Len(t, matches, 1, "the name is matched case-insensitively")
	require.Equal(t, ScopeGlobal, matches[0].Scope)
}

func TestRemove(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript, "two.sh": twoScript})

	install(t, root, f, testRepo)
	src, _, err := ParseSource(testRepo)
	require.NoError(t, err)

	m, err := Remove(root, src)
	require.NoError(t, err)
	require.Equal(t, commitOne, m.Commit)
	require.NoDirExists(t, Dir(root, src))

	_, err = Remove(root, src)
	require.ErrorIs(t, err, ErrNotInstalled)
}
