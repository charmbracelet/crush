package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A command that runs another command must not skip the prompt just because
// its first word looks harmless: `env rm x` is an rm.
func TestBashTool_WrapperCommandsRequirePermission(t *testing.T) {
	for _, wrapper := range []string{
		"env", "env FOO=1", "env -i",
		"nice", "nice -n 5",
		"nohup",
		"time", "time -p",
		"timeout 5", "timeout -s KILL 5",
		"env timeout 5",
	} {
		t.Run(wrapper, func(t *testing.T) {
			canary := filepath.Join(t.TempDir(), "canary")
			require.NoError(t, os.WriteFile(canary, []byte("x"), 0o644))
			tool, perms := newBashToolWithRecordingPerms(t.TempDir(), false)
			ctx := context.WithValue(t.Context(), SessionIDContextKey, "s")

			resp := runBashTool(t, tool, ctx, BashParams{Description: "wrapper", Command: wrapper + " rm " + canary})

			require.Equal(t, 1, perms.requestCount, "wrapper must ask for permission")
			require.Contains(t, resp.Content, "User denied permission")
			require.FileExists(t, canary, "denied command must not run")
		})
	}
}

func TestBashTool_ReadOnlyCommandsStillAutoApprove(t *testing.T) {
	for _, cmd := range []string{
		"ls", "ls -la", "pwd", "echo hi", "date", "whoami", "uname -a", "printenv HOME",
		"git status", "git log --oneline -1", "git diff",
		"git branch", "git branch -a", "git branch -r", "git branch --list", "git branch -v",
		"git tag", "git tag -l", "git tag --list",
		"git remote", "git remote -v", "git remote show", "git remote show origin",
		"git config --get-regexp no-such-key",
	} {
		t.Run(cmd, func(t *testing.T) {
			tool, perms := newBashToolWithRecordingPerms(t.TempDir(), false)
			ctx := context.WithValue(t.Context(), SessionIDContextKey, "s")

			resp := runBashTool(t, tool, ctx, BashParams{Description: "read-only", Command: cmd})

			require.Equal(t, 0, perms.requestCount, "read-only command should not prompt")
			require.NotContains(t, resp.Content, "User denied permission")
		})
	}
}

// A safe first word must not skip the prompt when the shell can do more than
// that word: redirection writes files, `&` and newlines start another command.
func TestBashTool_RedirectBackgroundNewlineRequirePermission(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cmd     string // %s is the target file
		creates bool   // the command would create the file; otherwise it would remove it
	}{
		{"redirect out", "echo x > %s", true},
		{"redirect append", "echo x >> %s", true},
		{"redirect both", "echo x &> %s", true},
		{"redirect in", "ls < %s", false},
		{"background", "ls & rm %s", false},
		{"newline", "ls -l\nrm %s", false},
		{"carriage return", "ls -l\rrm %s", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "target")
			if !tc.creates {
				require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
			}
			tool, perms := newBashToolWithRecordingPerms(t.TempDir(), false)
			ctx := context.WithValue(t.Context(), SessionIDContextKey, "s")

			resp := runBashTool(t, tool, ctx, BashParams{Description: "meta", Command: fmt.Sprintf(tc.cmd, file)})

			require.Equal(t, 1, perms.requestCount, "must ask for permission")
			require.Contains(t, resp.Content, "User denied permission")
			if tc.creates {
				require.NoFileExists(t, file, "denied command must not run")
			} else {
				require.FileExists(t, file, "denied command must not run")
			}
		})
	}
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

// Safe-listed commands that can still write, delete or kill must prompt:
// git --output writes a file, branch/tag/remote have mutating forms, kill and
// killall signal any process. The permission service always denies, so the
// repo and the canary file must come out unchanged.
func TestBashTool_GitWriteFormsAndKillRequirePermission(t *testing.T) {
	for _, cmd := range []string{
		"git diff --output=%s", "git diff --output %s", "git diff HEAD --output=%s",
		"git log --output=%s", "git show --output=%s", "git diff --output-indicator-new=x --output=%s",
		`git diff --out""put=%s`, "git diff --{output=%s,}", "git diff --$(echo output)=%s",
		"git branch -D keep", "git branch -d keep", "git branch -m keep other", "git branch -a newb", "git branch newb",
		"git tag -d v1", "git tag newt", "git tag -l -d v1", "git tag -a newt -m x",
		"git remote add o2 u", "git remote remove origin", "git remote rm origin", "git remote set-url origin u2",
		"git remote rename origin o3", "git remote prune origin",
		"kill -0 1", "kill -0 -1", "killall -0 no-such-process",
	} {
		t.Run(cmd, func(t *testing.T) {
			repo := t.TempDir()
			gitIn(t, repo, "init", "-q")
			gitIn(t, repo, "commit", "-q", "--allow-empty", "-m", "m")
			gitIn(t, repo, "branch", "keep")
			gitIn(t, repo, "tag", "v1")
			gitIn(t, repo, "remote", "add", "origin", "u")
			out := filepath.Join(t.TempDir(), "canary")
			require.NoError(t, os.WriteFile(out, []byte("x"), 0o644))
			before := gitIn(t, repo, "for-each-ref") + gitIn(t, repo, "remote", "-v")

			tool, perms := newBashToolWithRecordingPerms(repo, false)
			ctx := context.WithValue(t.Context(), SessionIDContextKey, "s")
			resp := runBashTool(t, tool, ctx, BashParams{Description: "write form", Command: strings.ReplaceAll(cmd, "%s", out)})

			require.Equal(t, 1, perms.requestCount, "must ask for permission")
			require.Contains(t, resp.Content, "User denied permission")
			got, err := os.ReadFile(out)
			require.NoError(t, err)
			require.Equal(t, "x", string(got), "denied command must not write the file")
			require.Equal(t, before, gitIn(t, repo, "for-each-ref")+gitIn(t, repo, "remote", "-v"), "denied command must not change refs/remotes")
		})
	}
}

// Read-only git verbs have forms that run a program: a pager or upload-pack
// named on the command line (also abbreviated or bundled, as git accepts),
// an external diff or textconv driver, or config/env set for the call. They
// must prompt; the program here is always `true`, and the permission service
// denies.
func TestBashTool_GitFormsThatRunProgramsRequirePermission(t *testing.T) {
	for _, cmd := range []string{
		"git grep -Otrue hello", "git grep -iOtrue hello",
		"git grep --open-files-in-pager=true hello", "git grep --open=true hello",
		"git grep --textconv hello", "git grep --textc hello",
		"git ls-remote --upload-pack=true .", "git ls-remote --upload-pack true .",
		"git ls-remote --upload=true .", "git ls-remote --exec=true .",
		"git diff --ext-diff", "git log -p --ext-diff -1", "git show --ext-diff", "git diff --textconv",
		"git -c core.pager=true log", "git --config-env=core.pager=HOME log", "git --exec-path=. log",
		"GIT_PAGER=true git log", "GIT_EXTERNAL_DIFF=true git diff",
	} {
		t.Run(cmd, func(t *testing.T) {
			repo := t.TempDir()
			gitIn(t, repo, "init", "-q")
			require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("hello\n"), 0o644))
			gitIn(t, repo, "add", "a.txt")
			gitIn(t, repo, "commit", "-q", "-m", "m")

			tool, perms := newBashToolWithRecordingPerms(repo, false)
			ctx := context.WithValue(t.Context(), SessionIDContextKey, "s")
			resp := runBashTool(t, tool, ctx, BashParams{Description: "runs a program", Command: cmd})

			require.Equal(t, 1, perms.requestCount, "must ask for permission")
			require.Contains(t, resp.Content, "User denied permission")
		})
	}
}

// The checks above must not catch the plain read-only forms they resemble.
func TestBashTool_GitReadOnlyLookalikesStillAutoApprove(t *testing.T) {
	for _, cmd := range []string{
		"git grep -o hello", "git grep -n hello", "git grep -c hello", "git log --oneline -1",
		"git diff --no-ext-diff", "git log -p -1 --no-textconv", "git ls-remote --heads .",
	} {
		t.Run(cmd, func(t *testing.T) {
			repo := t.TempDir()
			gitIn(t, repo, "init", "-q")
			require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("hello\n"), 0o644))
			gitIn(t, repo, "add", "a.txt")
			gitIn(t, repo, "commit", "-q", "-m", "m")

			tool, perms := newBashToolWithRecordingPerms(repo, false)
			ctx := context.WithValue(t.Context(), SessionIDContextKey, "s")
			resp := runBashTool(t, tool, ctx, BashParams{Description: "read-only", Command: cmd})

			require.Equal(t, 0, perms.requestCount, "read-only form should not prompt")
			require.NotContains(t, resp.Content, "User denied permission")
		})
	}
}

// "git <verb>-<x>" is not the safe verb but an alias or a git-<verb>-<x>
// program on PATH, so it prompts. Here the repo's alias would touch a canary.
func TestBashTool_GitVerbLookalikeRequiresPermission(t *testing.T) {
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q")
	canary := filepath.Join(t.TempDir(), "canary")
	gitIn(t, repo, "config", "alias.log-x", "!touch "+canary)

	tool, perms := newBashToolWithRecordingPerms(repo, false)
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "s")
	resp := runBashTool(t, tool, ctx, BashParams{Description: "alias", Command: "git log-x"})

	require.Equal(t, 1, perms.requestCount, "must ask for permission")
	require.Contains(t, resp.Content, "User denied permission")
	require.NoFileExists(t, canary, "denied command must not run")
}
