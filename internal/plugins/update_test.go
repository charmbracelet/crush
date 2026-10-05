package plugins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustSource(t *testing.T, spec string) Source {
	t.Helper()
	src, _, err := ParseSource(spec)
	require.NoError(t, err)
	return src
}

func TestUpdateMovesToTheNewCommit(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript, "two.sh": twoScript})
	install(t, root, f, testRepo)

	// Upstream pushes: a changed file and a deleted one.
	f.publish(testRepo, DefaultRef, commitTwo, map[string]string{"one.sh": strings.Join([]string{oneScript, "# touched"}, "\n")})

	res, err := Update(context.Background(), UpdateOptions{Root: root, Source: mustSource(t, testRepo), GitHub: f.client()})
	require.NoError(t, err)
	require.False(t, res.UpToDate)
	require.Equal(t, commitOne, res.Previous)
	require.Equal(t, commitTwo, res.Manifest.Commit)
	require.Equal(t, []string{"one.sh"}, res.Changed)
	require.Equal(t, []string{"two.sh"}, res.Removed)

	dir := Dir(root, mustSource(t, testRepo))
	body, err := os.ReadFile(filepath.Join(dir, "one.sh"))
	require.NoError(t, err)
	require.Contains(t, string(body), "# touched")
	require.NoFileExists(t, filepath.Join(dir, "two.sh"), "a plugin deleted upstream must not keep running")
	require.Equal(t, commitTwo, mustLoad(t, dir).Commit)
}

func TestUpdateKeepsTheInstallDate(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	first := install(t, root, f, testRepo)

	f.publish(testRepo, DefaultRef, commitTwo, map[string]string{"one.sh": twoScript})
	res, err := Update(context.Background(), UpdateOptions{Root: root, Source: mustSource(t, testRepo), GitHub: f.client()})
	require.NoError(t, err)
	require.Equal(t, first.Manifest.InstalledAt, res.Manifest.InstalledAt)
	require.False(t, res.Manifest.UpdatedAt.Before(first.Manifest.UpdatedAt))
}

func TestUpdateRefusesLocalChanges(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	install(t, root, f, testRepo)
	f.publish(testRepo, DefaultRef, commitTwo, map[string]string{"one.sh": twoScript})

	dir := Dir(root, mustSource(t, testRepo))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "one.sh"), []byte("provider add edited"), scriptPerm))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.sh"), []byte(oneScript), scriptPerm))

	src := mustSource(t, testRepo)
	_, err := Update(context.Background(), UpdateOptions{Root: root, Source: src, GitHub: f.client()})
	require.ErrorContains(t, err, "has local changes")
	require.ErrorContains(t, err, "crush plugin trust example/crush-plugins")
	require.ErrorContains(t, err, "--force")
	require.Equal(t, commitOne, mustLoad(t, dir).Commit, "a refused update must touch nothing")

	// Trust accepts the local state, and the install is clean again.
	m, diff, err := Trust(root, src)
	require.NoError(t, err)
	require.Equal(t, []string{"extra.sh"}, diff.Added)
	require.Equal(t, []string{"one.sh"}, diff.Changed)
	require.Equal(t, commitOne, m.Commit, "trusting keeps the commit provenance")
	require.Len(t, m.Files, 2)

	installed, err := Discover([]ScopedRoot{{Scope: ScopeGlobal, Path: root}})
	require.NoError(t, err)
	require.Len(t, installed, 1)
	require.False(t, installed[0].Modified)

	// Now the update proceeds, and it drops the file the user added because
	// the new commit does not contain it.
	res, err := Update(context.Background(), UpdateOptions{Root: root, Source: src, GitHub: f.client()})
	require.NoError(t, err)
	require.Equal(t, commitTwo, res.Manifest.Commit)
	require.Equal(t, []string{"extra.sh"}, res.Removed)
	require.NoFileExists(t, filepath.Join(dir, "extra.sh"))
}

func TestUpdateForceOverwritesLocalChanges(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	install(t, root, f, testRepo)
	f.publish(testRepo, DefaultRef, commitTwo, map[string]string{"one.sh": twoScript})

	dir := Dir(root, mustSource(t, testRepo))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "one.sh"), []byte("provider add edited"), scriptPerm))

	res, err := Update(context.Background(), UpdateOptions{Root: root, Source: mustSource(t, testRepo), Force: true, GitHub: f.client()})
	require.NoError(t, err)
	require.Equal(t, commitTwo, res.Manifest.Commit)
	body, err := os.ReadFile(filepath.Join(dir, "one.sh"))
	require.NoError(t, err)
	require.Equal(t, twoScript, string(body))
}

func TestUpdateNothingChanged(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	install(t, root, f, testRepo)

	res, err := Update(context.Background(), UpdateOptions{Root: root, Source: mustSource(t, testRepo), GitHub: f.client()})
	require.NoError(t, err)
	require.True(t, res.UpToDate)
	require.Equal(t, commitOne, res.Manifest.Commit)
}

func TestUpdateCanRetargetTheRef(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	f.publish(testRepo, "release/2", commitV2, map[string]string{"one.sh": twoScript})
	install(t, root, f, testRepo)

	res, err := Update(context.Background(), UpdateOptions{
		Root:   root,
		Source: mustSource(t, testRepo),
		Ref:    "release/2",
		GitHub: f.client(),
	})
	require.NoError(t, err)
	require.Equal(t, "release/2", res.Manifest.Ref)
	require.Equal(t, commitV2, res.Manifest.Commit)

	// Retargeting to a ref that resolves to the same commit still records the
	// new ref, so the next update follows it.
	res, err = Update(context.Background(), UpdateOptions{
		Root:   root,
		Source: mustSource(t, testRepo),
		Ref:    "release/2",
		GitHub: f.client(),
	})
	require.NoError(t, err)
	require.True(t, res.UpToDate)
	require.Equal(t, "release/2", mustLoad(t, Dir(root, mustSource(t, testRepo))).Ref)
}

func TestUpdateReportsMissingInstall(t *testing.T) {
	root := t.TempDir()
	_, err := Update(context.Background(), UpdateOptions{Root: root, Source: mustSource(t, testRepo), GitHub: newFakeGitHub(t).client()})
	require.ErrorIs(t, err, ErrNotInstalled)
}

func TestUpdateAllContinuesPastOneFailure(t *testing.T) {
	global := t.TempDir()
	f := newFakeGitHub(t)
	f.publish("good/plugins", DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	f.publish("bad/plugins", DefaultRef, commitTwo, map[string]string{"two.sh": twoScript})
	install(t, global, f, "good/plugins")
	install(t, global, f, "bad/plugins")

	// Only the second repository goes dark.
	f.status["/repos/bad/plugins/commits/"+DefaultRef] = 500

	outcomes, err := UpdateAll(context.Background(), []ScopedRoot{{Scope: ScopeGlobal, Path: global}}, f.client(), false)
	require.NoError(t, err)
	require.Len(t, outcomes, 2)

	bySource := map[string]Outcome{}
	for _, o := range outcomes {
		bySource[o.Source.String()] = o
	}
	require.NoError(t, bySource["good/plugins"].Err)
	require.ErrorContains(t, bySource["bad/plugins"].Err, "returned 500")
}

func TestTrustNeedsSomethingToTrust(t *testing.T) {
	root := t.TempDir()
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, map[string]string{"one.sh": oneScript})
	install(t, root, f, testRepo)

	require.NoError(t, os.Remove(filepath.Join(Dir(root, mustSource(t, testRepo)), "one.sh")))
	_, _, err := Trust(root, mustSource(t, testRepo))
	require.ErrorContains(t, err, "no .sh files to trust")
}

func TestTrustUninstalledFails(t *testing.T) {
	_, _, err := Trust(t.TempDir(), mustSource(t, testRepo))
	require.ErrorIs(t, err, ErrNoManifest)
}

func mustLoad(t *testing.T, dir string) Manifest {
	t.Helper()
	m, err := LoadManifest(dir)
	require.NoError(t, err)
	return m
}
