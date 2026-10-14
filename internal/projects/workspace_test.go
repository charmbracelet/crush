package projects

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// No t.Parallel here: the tests share process-wide env (t.Setenv).

func setupRegistry(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmp, "data"))
	return tmp
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func realDir(t *testing.T, dir string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return r
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	git(t, dir, "init", "-q")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
}

func TestSubdirSharesEntryWithRepoRoot(t *testing.T) {
	tmp := setupRegistry(t)
	repo := filepath.Join(tmp, "repo")
	initRepo(t, repo)
	sub := filepath.Join(repo, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	require.NoError(t, Register(repo, "d1"))
	require.NoError(t, Register(sub, "d1"))

	got, err := List()
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, realDir(t, repo), got[0].Path)
}

func TestTwoWorktreesAreTwoEntries(t *testing.T) {
	tmp := setupRegistry(t)
	repo := filepath.Join(tmp, "repo")
	initRepo(t, repo)
	wt := filepath.Join(tmp, "wt")
	git(t, repo, "worktree", "add", "-q", "-b", "feature", wt)

	require.NoError(t, Register(repo, "d1"))
	require.NoError(t, Register(wt, "d2"))

	got, err := List()
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, realDir(t, wt), got[0].Path)
	require.Equal(t, realDir(t, repo), got[1].Path)
}

func TestNormalizeNonGitDirAndSymlink(t *testing.T) {
	tmp := setupRegistry(t)
	plain := filepath.Join(tmp, "plain", "x")
	require.NoError(t, os.MkdirAll(plain, 0o755))
	link := filepath.Join(tmp, "link")
	require.NoError(t, os.Symlink(filepath.Join(tmp, "plain"), link))

	got, err := NormalizeWorkspace(filepath.Join(link, "x"))
	require.NoError(t, err)
	require.Equal(t, realDir(t, plain), got)

	_, err = NormalizeWorkspace(filepath.Join(tmp, "missing"))
	require.Error(t, err)
}

func TestReadsOldFormatFile(t *testing.T) {
	setupRegistry(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(projectsFilePath()), 0o700))
	old := `{"projects":[{"path":"/old/p","data_dir":"/old/p/.crush","last_accessed":"2026-01-02T03:04:05Z"}]}`
	require.NoError(t, os.WriteFile(projectsFilePath(), []byte(old), 0o600))

	got, err := List()
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, currentHost(), got[0].Host)
	require.False(t, got[0].Pinned)

	// A new registration keeps the old entry (history is never trimmed).
	require.NoError(t, Register("/new/p", "/new/p/.crush"))
	got, err = List()
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestPinAndRemoveLeaveDataDirUntouched(t *testing.T) {
	tmp := setupRegistry(t)
	proj := filepath.Join(tmp, "proj")
	dataDir := filepath.Join(proj, ".crush")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	marker := filepath.Join(dataDir, "crush.db")
	require.NoError(t, os.WriteFile(marker, []byte("data"), 0o600))

	require.NoError(t, Register(proj, dataDir))
	require.NoError(t, Register(filepath.Join(tmp, "other"), "x"))
	require.NoError(t, Pin(proj, true))

	got, err := List()
	require.NoError(t, err)
	require.True(t, got[1].Pinned)
	require.Equal(t, realDir(t, proj), got[1].Path, "Pin must not reorder")

	// Re-registering keeps the pin.
	require.NoError(t, Register(proj, dataDir))
	got, _ = List()
	require.True(t, got[0].Pinned)

	require.NoError(t, Pin(proj, false))
	require.NoError(t, Remove(proj))
	got, _ = List()
	require.Len(t, got, 1)

	b, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "data", string(b))
	require.Error(t, Remove(proj), "second Remove must report not found")
}

func TestRemoveStaleEntryWhoseDirIsGone(t *testing.T) {
	setupRegistry(t)
	require.NoError(t, Register("/gone/forever", "/gone/forever/.crush"))
	require.NoError(t, Remove("/gone/forever"))
	got, _ := List()
	require.Empty(t, got)
}

// TestConcurrentWriters re-executes this test binary as separate processes
// that each register distinct paths at once; none may be lost.
func TestConcurrentWriters(t *testing.T) {
	if id := os.Getenv("PROJECTS_TEST_WRITER"); id != "" {
		for i := range 25 {
			if err := Register(fmt.Sprintf("/w%s/p%d", id, i), "d"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		return
	}
	setupRegistry(t)
	const procs = 4
	cmds := make([]*exec.Cmd, procs)
	for i := range cmds {
		cmds[i] = exec.Command(os.Args[0], "-test.run=^TestConcurrentWriters$")
		cmds[i].Env = append(os.Environ(), "PROJECTS_TEST_WRITER="+strconv.Itoa(i))
		require.NoError(t, cmds[i].Start())
	}
	for _, c := range cmds {
		require.NoError(t, c.Wait())
	}

	data, err := os.ReadFile(projectsFilePath())
	require.NoError(t, err)
	var list ProjectList
	require.NoError(t, json.Unmarshal(data, &list), "registry must be valid JSON")
	require.Len(t, list.Projects, procs*25)
	entries, err := filepath.Glob(filepath.Join(filepath.Dir(projectsFilePath()), "*.tmp-*"))
	require.NoError(t, err)
	require.Empty(t, entries, "no temp files left behind")
}

func TestSubdirThenRootKeepsBothDataDirs(t *testing.T) {
	tmp := setupRegistry(t)
	repo := filepath.Join(tmp, "repo")
	initRepo(t, repo)
	sub := filepath.Join(repo, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	subData := filepath.Join(sub, ".crush")
	repoData := filepath.Join(repo, ".crush")

	require.NoError(t, Register(sub, subData))
	require.NoError(t, Register(repo, repoData))

	got, err := List()
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, repoData, got[0].DataDir, "DataDir is the most recent")
	require.Equal(t, []string{repoData, subData}, got[0].DataDirs())
	require.Len(t, got[0].Locations, 2)
	require.Equal(t, realDir(t, sub), got[0].Locations[1].Cwd)

	// Same data dir again: deduped, moved to the front.
	require.NoError(t, Register(sub, subData))
	got, _ = List()
	require.Equal(t, []string{subData, repoData}, got[0].DataDirs())
	require.Len(t, got[0].Locations, 2)
}

func TestLocationsCappedOldestDropped(t *testing.T) {
	setupRegistry(t)
	for i := range maxLocations + 3 {
		require.NoError(t, Register("/proj", fmt.Sprintf("/d%d", i)))
	}
	got, err := List()
	require.NoError(t, err)
	require.Len(t, got[0].Locations, maxLocations)
	dirs := got[0].DataDirs()
	require.Len(t, dirs, maxLocations)
	require.Equal(t, fmt.Sprintf("/d%d", maxLocations+2), dirs[0])
	require.Equal(t, "/d3", dirs[len(dirs)-1], "d0..d2 are the oldest and dropped")
}

func TestOldFormatEntryKeepsItsDataDirOnNextRegister(t *testing.T) {
	setupRegistry(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(projectsFilePath()), 0o700))
	old := `{"projects":[{"path":"/old/p","data_dir":"/old/p/.crush","last_accessed":"2026-01-02T03:04:05Z"}]}`
	require.NoError(t, os.WriteFile(projectsFilePath(), []byte(old), 0o600))

	got, err := List()
	require.NoError(t, err)
	require.Empty(t, got[0].Locations)
	require.Equal(t, []string{"/old/p/.crush"}, got[0].DataDirs())

	require.NoError(t, Register("/old/p", "/old/p/sub/.crush"))
	got, _ = List()
	require.Equal(t, []string{"/old/p/sub/.crush", "/old/p/.crush"}, got[0].DataDirs())
}

func TestRelativePathStoredAbsoluteEvenWhenMissing(t *testing.T) {
	tmp := setupRegistry(t)
	t.Chdir(tmp)
	require.NoError(t, Register("does/not/exist", "d"))
	got, err := List()
	require.NoError(t, err)
	require.True(t, filepath.IsAbs(got[0].Path), "stored %q", got[0].Path)
	require.Equal(t, filepath.Join(realDir(t, tmp), "does/not/exist"), got[0].Path)
}

func TestRegisterThroughSymlinkedRegistry(t *testing.T) {
	tmp := setupRegistry(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(projectsFilePath()), 0o700))
	dotfiles := filepath.Join(tmp, "dotfiles")
	require.NoError(t, os.MkdirAll(dotfiles, 0o755))
	target := filepath.Join(dotfiles, "projects.json")
	require.NoError(t, os.Symlink(target, projectsFilePath())) // dangling at first

	require.NoError(t, Register("/sym/a", "d"))
	require.NoError(t, Register("/sym/b", "d"))

	fi, err := os.Lstat(projectsFilePath())
	require.NoError(t, err)
	require.NotZero(t, fi.Mode()&os.ModeSymlink, "registry must stay a symlink")
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	var list ProjectList
	require.NoError(t, json.Unmarshal(data, &list))
	require.Len(t, list.Projects, 2)
	entries, _ := filepath.Glob(filepath.Join(dotfiles, "*.tmp-*"))
	require.Empty(t, entries)
}

// TestCorruptRegistryIsNeverOverwritten pins the no-self-heal rule: a registry
// that cannot be parsed may hold the user's only copy of their history, so
// every writer refuses and leaves the bytes alone.
func TestCorruptRegistryIsNeverOverwritten(t *testing.T) {
	setupRegistry(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(projectsFilePath()), 0o700))
	bad := []byte(`{"projects":[{"path":"/keep/me"`)
	require.NoError(t, os.WriteFile(projectsFilePath(), bad, 0o600))

	for name, op := range map[string]func() error{
		"Register": func() error { return Register("/x", "d") },
		"Save":     func() error { return Save(&ProjectList{}) },
		"Pin":      func() error { return Pin("/keep/me", true) },
		"Remove":   func() error { return Remove("/keep/me") },
	} {
		require.Error(t, op(), name)
		got, err := os.ReadFile(projectsFilePath())
		require.NoError(t, err)
		require.Equal(t, string(bad), string(got), "%s must leave the corrupt file untouched", name)
	}
}

// TestPinRemoveNormalizeOutsideLocks: the fallback path of find runs git
// (up to 5s); it must not do so while holding the registry locks.
func TestPinRemoveNormalizeOutsideLocks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a shell script as git")
	}
	tmp := setupRegistry(t)
	bin := filepath.Join(tmp, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	marker := filepath.Join(tmp, "git-ran")
	script := "#!/bin/sh\ntouch " + marker + "\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := filepath.Join(tmp, "unregistered")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	for name, op := range map[string]func() error{
		"Pin":    func() error { return Pin(dir, true) },
		"Remove": func() error { return Remove(dir) },
	} {
		require.NoError(t, os.RemoveAll(marker))
		mu.Lock() // stands for another goroutine inside update()
		done := make(chan error, 1)
		go func() { done <- op() }()
		require.Eventually(t, func() bool { _, err := os.Stat(marker); return err == nil },
			3*time.Second, 10*time.Millisecond, "%s must run git before taking the lock", name)
		mu.Unlock()
		require.ErrorContains(t, <-done, "not found")
	}
}
