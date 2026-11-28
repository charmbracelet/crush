package plugins

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/fsext"
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

// A publish that fails leaves the previous release, or nothing, in place: the
// install directory never shows a partial or unrecorded release.
func TestInstallFailurePublishesNothing(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, root string, f *fakeGitHub)
		want    []string
	}{
		{name: "first install", prepare: func(*testing.T, string, *fakeGitHub) {}},
		{
			name: "update",
			prepare: func(t *testing.T, root string, f *fakeGitHub) {
				f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript, "two.sh": twoScript})
				install(t, root, f, testRepo)
			},
			want: []string{"one.sh", "two.sh"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			f := newFakeGitHub(t)
			tt.prepare(t, root, f)
			f.publish(testRepo, DefaultRef, commitTwo, map[string]string{"one.sh": twoScript, "three.sh": oneScript})
			dir := Dir(root, mustSource(t, testRepo))

			// The staged release must be complete when it would become visible.
			renameDir = func(from, to string) error {
				if to == dir && !strings.HasSuffix(from, ".old") {
					staged := mustLoad(t, from)
					require.Equal(t, commitTwo, staged.Commit)
					diff, err := staged.Diff(from)
					require.NoError(t, err)
					require.True(t, diff.Empty(), "staged release: %+v", diff)
					return errors.New("injected publish failure")
				}
				return os.Rename(from, to)
			}
			t.Cleanup(func() { renameDir = fsext.Rename })

			_, err := installErr(t, root, f, testRepo)
			require.ErrorContains(t, err, "injected publish failure")

			entries, err := os.ReadDir(root)
			require.NoError(t, err)
			if tt.want == nil {
				require.Empty(t, entries, "a failed first install must leave nothing behind")
				return
			}
			require.Len(t, entries, 1, "only the previous release may remain")
			previous := mustLoad(t, dir)
			require.Equal(t, commitOne, previous.Commit)
			require.Equal(t, tt.want, previous.FileNames())
			diff, err := previous.Diff(dir)
			require.NoError(t, err)
			require.True(t, diff.Empty(), "the previous release must be intact: %+v", diff)
		})
	}
}

// When the new release cannot be renamed in and the old one cannot be put
// back, the error names where the old release survives, and stale staging
// directories from earlier failed runs are removed.
func TestInstallNamesPreservedReleaseWhenRollbackFails(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	install(t, root, f, testRepo)
	f.publish(testRepo, DefaultRef, commitTwo, map[string]string{"one.sh": twoScript})
	src := mustSource(t, testRepo)
	dir := Dir(root, src)

	stale := filepath.Join(root, "."+src.DirName()+".stage-old")
	require.NoError(t, os.MkdirAll(stale, dirPerm))
	unrelated := filepath.Join(root, ".other__repo.stage-1")
	require.NoError(t, os.MkdirAll(unrelated, dirPerm))

	renameDir = func(from, to string) error {
		if to == dir {
			return errors.New("injected rename failure")
		}
		return os.Rename(from, to)
	}
	t.Cleanup(func() { renameDir = fsext.Rename })

	_, publishErr := installErr(t, root, f, testRepo)
	require.ErrorContains(t, publishErr, "previous release preserved at")
	require.NoDirExists(t, stale)
	require.DirExists(t, unrelated)

	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	var preserved string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".old") {
			preserved = filepath.Join(root, e.Name())
		}
	}
	require.NotEmpty(t, preserved)
	require.ErrorContains(t, publishErr, preserved)
	require.Equal(t, commitOne, mustLoad(t, preserved).Commit)
}

// Publishing replaces the whole directory, so one the installer does not own
// is refused instead of deleted.
func TestInstallRefusesUnmanagedDirectory(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	dir := Dir(root, mustSource(t, testRepo))
	require.NoError(t, os.MkdirAll(dir, dirPerm))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mine.sh"), []byte(twoScript), scriptPerm))

	_, err := installErr(t, root, f, testRepo)
	require.ErrorContains(t, err, "not a managed install")
	require.FileExists(t, filepath.Join(dir, "mine.sh"))
	require.NoFileExists(t, filepath.Join(dir, "one.sh"))
}

func TestScripts(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	for _, name := range []string{
		"z.sh", "a.sh", ".hidden.sh", "notes.txt",
		"repo/b.sh", "repo/.plugin.json", "repo/deep/c.sh",
		".repo.stage-1/d.sh",
	} {
		path := filepath.Join(global, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), dirPerm))
		require.NoError(t, os.WriteFile(path, nil, scriptPerm))
	}
	require.NoError(t, os.WriteFile(filepath.Join(project, "p.sh"), nil, scriptPerm))

	require.Equal(t, []string{
		filepath.Join(global, "a.sh"),
		filepath.Join(global, "repo", "b.sh"),
		filepath.Join(global, "z.sh"),
		filepath.Join(project, "p.sh"),
	}, Scripts(global, filepath.Join(t.TempDir(), "missing"), project))
}
