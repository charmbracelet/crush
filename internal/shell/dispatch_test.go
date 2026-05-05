package shell

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// writeScript is a small helper that drops a file with the given contents
// and executable mode into dir. Tests that need exec semantics rely on the
// 0o755 mode on Unix; Windows ignores file modes but doesn't need them
// because dispatch decides what to do from file contents, not permissions.
func writeScript(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return filepath.ToSlash(path)
}

// randSuffix returns a short random hex string, used to build
// intentionally-unique paths that won't collide with anything on disk.
func randSuffix() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// TestIsPathPrefixed covers the classification rules used by the dispatch
// handler to decide whether argv[0] is a file reference.
func TestIsPathPrefixed(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"./foo.sh", true},
		{"../foo.sh", true},
		{"/usr/bin/foo", true},
		{"foo", false},
		{"foo.sh", false},
		{"jq", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isPathPrefixed(c.in); got != c.want {
			t.Errorf("isPathPrefixed(%q) = %v, want %v", c.in, got, c.want)
		}
	}

	if runtime.GOOS == "windows" {
		winCases := []struct {
			in   string
			want bool
		}{
			{`C:\foo\bar.exe`, true},
			{`C:/foo/bar.exe`, true},
			{`c:\foo`, true},
			{`Z:/x`, true},
			{`C:`, false}, // just a drive, no path.
			{`\\server\share`, true},
		}
		for _, c := range winCases {
			if got := isPathPrefixed(c.in); got != c.want {
				t.Errorf("isPathPrefixed(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}

// TestParseShebang covers the shebang grammar: literal paths, env,
// env -S, kernel single-arg semantics, CRLF tolerance, and every
// enumerated error case.
func TestParseShebang(t *testing.T) {
	type want struct {
		interp string
		args   []string
		errSub string // substring expected in error message (empty → no error)
	}
	cases := []struct {
		name string
		in   string
		want want
	}{
		{
			name: "literal-no-args",
			in:   "#!/bin/bash\necho body\n",
			want: want{interp: "/bin/bash"},
		},
		{
			name: "literal-kernel-single-arg",
			in:   "#!/bin/bash -x -y\n",
			want: want{interp: "/bin/bash", args: []string{"-x -y"}},
		},
		{
			name: "env-basic",
			in:   "#!/usr/bin/env bash\n",
			want: want{interp: "bash"},
		},
		{
			name: "env-kernel-single-arg",
			in:   "#!/usr/bin/env bash -x\n",
			want: want{interp: "bash", args: []string{"-x"}},
		},
		{
			name: "env-dash-S-splits",
			in:   "#!/usr/bin/env -S bash -x\n",
			want: want{interp: "bash", args: []string{"-x"}},
		},
		{
			name: "env-dash-S-multi-args",
			in:   "#!/usr/bin/env -S bash -x --noprofile\n",
			want: want{interp: "bash", args: []string{"-x", "--noprofile"}},
		},
		{
			name: "leading-space",
			in:   "#! /usr/bin/env bash\n",
			want: want{interp: "bash"},
		},
		{
			name: "crlf",
			in:   "#!/bin/bash\r\n",
			want: want{interp: "/bin/bash"},
		},
		{
			name: "bare-env-name",
			in:   "#!env bash\n",
			want: want{interp: "bash"},
		},
		{
			name: "empty-after-hashbang",
			in:   "#!\n",
			want: want{errSub: "empty shebang"},
		},
		{
			name: "env-alone",
			in:   "#!/usr/bin/env\n",
			want: want{errSub: "missing program name"},
		},
		{
			name: "env-dash-S-alone",
			in:   "#!/usr/bin/env -S\n",
			want: want{errSub: "env -S requires a program"},
		},
		{
			name: "env-unknown-flag",
			in:   "#!/usr/bin/env -x bash\n",
			want: want{errSub: "unsupported env flag"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sb, err := parseShebang([]byte(c.in))
			if c.want.errSub != "" {
				if err == nil || !strings.Contains(err.Error(), c.want.errSub) {
					t.Fatalf("expected error containing %q, got: %v", c.want.errSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sb.interpreter != c.want.interp {
				t.Errorf("interpreter = %q, want %q", sb.interpreter, c.want.interp)
			}
			if !equalStringSlice(sb.args, c.want.args) {
				t.Errorf("args = %v, want %v", sb.args, c.want.args)
			}
		})
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// TestIsBinary covers the NUL-byte and magic-byte classification used to
// keep compiled executables off the in-process shell-source path.
func TestIsBinary(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want bool
	}{
		{"shell", []byte("echo hi\n"), false},
		{"nul", []byte("hello\x00world"), true},
		{"elf", []byte{0x7F, 'E', 'L', 'F', 0x02, 0x01}, true},
		{"mz", []byte("MZ\x90\x00"), true},
		{"macho-64-le", []byte{0xCF, 0xFA, 0xED, 0xFE}, true},
		{"short-non-binary", []byte("a"), false},
	}
	for _, c := range cases {
		if got := isBinary(c.in); got != c.want {
			t.Errorf("%s: isBinary = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestDispatch_ShellSourceNoShebang exercises the in-process shell-source
// branch: a file without a shebang runs via a nested runner and sees
// positional params from argv[1:].
func TestDispatch_ShellSourceNoShebang(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "args.sh", `echo "$1 $2"`)

	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: script + " alpha beta",
		Cwd:     dir,
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "alpha beta\n" {
		t.Fatalf("stdout = %q, want %q", got, "alpha beta\n")
	}
}

// TestDispatch_EmptyFile confirms a zero-byte script runs as empty shell
// source (exit 0, no output).
func TestDispatch_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "empty.sh", "")

	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: script,
		Cwd:     dir,
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v (stderr=%q)", err, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected empty output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

// TestDispatch_ShellSourceComposesWithPipe confirms the dispatch handler
// plays nicely with mvdan's pipeline logic: a shell-source script on the
// left feeds the jq builtin on the right.
func TestDispatch_ShellSourceComposesWithPipe(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "emit.sh", `printf '"value"'`)

	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: script + ` | jq -r .`,
		Cwd:     dir,
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "value\n" {
		t.Fatalf("stdout = %q, want %q", got, "value\n")
	}
}

// TestDispatch_MissingFile returns a clean error for a non-existent path.
func TestDispatch_MissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.sh")
	err := Run(t.Context(), RunOptions{
		Command: missing,
		Cwd:     dir,
	})
	if err == nil {
		t.Fatal("expected error for missing script, got nil")
	}
}

// TestDispatch_DirectoryNotFile surfaces a distinct error when the path
// resolves to a directory.
func TestDispatch_DirectoryNotFile(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "adir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "./adir",
		Cwd:     dir,
		Stderr:  &stderr,
	})
	if err == nil {
		t.Fatal("expected error when invoking a directory, got nil")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected 'is a directory' in error, got: %v", err)
	}
}

// TestDispatch_BashShebang runs a #!/bin/bash script via os/exec. Skipped
// if bash isn't available (rare in CI, but keep the test robust).
func TestDispatch_BashShebang(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash not in PATH: %v", err)
	}
	_ = bash

	dir := t.TempDir()
	script := writeScript(t, dir, "bash-echo.sh", "#!/usr/bin/env bash\necho bashout\n")

	var stdout, stderr bytes.Buffer
	err = Run(t.Context(), RunOptions{
		Command: script,
		Cwd:     dir,
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v (stderr=%q)", err, stderr.String())
	}
	if got := stdout.String(); got != "bashout\n" {
		t.Fatalf("stdout = %q, want %q", got, "bashout\n")
	}
}

// TestDispatch_ShebangPassesExitCode maps interpreter exit codes through to
// interp.ExitStatus so the caller can inspect them with ExitCode.
func TestDispatch_ShebangPassesExitCode(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not in PATH: %v", err)
	}
	dir := t.TempDir()
	script := writeScript(t, dir, "fail.sh", "#!/usr/bin/env bash\nexit 5\n")

	err := Run(t.Context(), RunOptions{
		Command: script,
		Cwd:     dir,
	})
	if err == nil {
		t.Fatal("expected non-nil error from exit 5")
	}
	if code := ExitCode(err); code != 5 {
		t.Fatalf("ExitCode = %d, want 5", code)
	}
}

// TestDispatch_MissingInterpreter surfaces a clear error (and non-zero
// exit) when the shebang points to a binary that doesn't exist and has
// no PATH fallback.
func TestDispatch_MissingInterpreter(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "bad.sh", "#!/no/such/interpreter-"+randSuffix()+"\n:\n")

	var stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: script,
		Cwd:     dir,
		Stderr:  &stderr,
	})
	if err == nil {
		t.Fatal("expected error for missing interpreter, got nil")
	}
	if ExitCode(err) == 0 {
		t.Fatalf("expected non-zero exit code, got 0")
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("expected 'not found' in stderr, got: %q", stderr.String())
	}
}

// TestDispatch_BarePathNotHandled confirms the handler ignores
// non-path-prefixed argv[0] entirely: a benign bare `true` command must
// not try to open a file in cwd. If dispatch were (incorrectly) firing
// on bare commands, this test would see probeFile's ENOENT.
func TestDispatch_BarePathNotHandled(t *testing.T) {
	dir := t.TempDir()
	err := Run(t.Context(), RunOptions{
		Command: "true",
		Cwd:     dir,
	})
	if err != nil {
		t.Fatalf("bare `true` should not trigger dispatch: %v", err)
	}
}

// TestDispatch_ProbeWindowClassifiesByHead confirms that classification is
// done on the first probeWindow bytes even when the file is much larger;
// a file whose head is shell source but whose tail contains NUL bytes is
// classified as shell source, not binary.
func TestDispatch_ProbeWindowClassifiesByHead(t *testing.T) {
	dir := t.TempDir()
	head := "echo prefix\n"
	// Pad past probeWindow, then append some NULs.
	padding := strings.Repeat(" ", probeWindow)
	contents := head + padding + "\x00\x00\x00"
	script := writeScript(t, dir, "long.sh", contents)

	var stdout bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: script,
		Cwd:     dir,
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); !strings.HasPrefix(got, "prefix\n") {
		t.Fatalf("stdout = %q, want prefix %q", got, "prefix\n")
	}
}

// TestDispatch_BinaryPassthroughExecutes copies a real binary from PATH
// into a tempdir, invokes it via a path-prefixed argv[0], and verifies it
// ran — i.e. the binary branch correctly returns through `next` to the
// default exec handler. We use whichever of `true`/`echo` is available on
// PATH so the test works on any Unix-y system; it skips on Windows where
// the stock binaries don't share names and the Go test binary approach
// is heavier than this test deserves.
func TestDispatch_BinaryPassthroughExecutes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on a Unix-style PATH binary")
	}
	src, err := exec.LookPath("true")
	if err != nil {
		t.Skipf("no `true` binary on PATH: %v", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "copied-true")
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}

	runErr := Run(t.Context(), RunOptions{
		Command: dst,
		Cwd:     dir,
		// Default handler needs PATH to resolve dynamic linker / loader
		// helpers on some systems; inherit the process env so the copy
		// can actually start.
		Env: os.Environ(),
	})
	if runErr != nil {
		t.Fatalf("expected copy of /bin/true to exit 0, got: %v", runErr)
	}
}

// TestDispatch_UnreadableFile confirms an EACCES on the script surfaces
// as a clean error rather than a silent fallback or a mis-classified
// shell-source attempt. POSIX-only: Windows doesn't have the same
// permission model and running as root would bypass the check anyway.
func TestDispatch_UnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission model")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file mode permission checks")
	}
	dir := t.TempDir()
	script := writeScript(t, dir, "unreadable.sh", "echo nope\n")
	if err := os.Chmod(script, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(script, 0o644) })

	err := Run(t.Context(), RunOptions{
		Command: script,
		Cwd:     dir,
	})
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
	if !strings.Contains(err.Error(), "permission") {
		t.Fatalf("expected 'permission' in error, got: %v", err)
	}
}

// TestDispatch_SymlinkLoop confirms that an ELOOP-returning path surfaces
// cleanly. POSIX-only: creating symlinks reliably on Windows requires
// elevated privileges or developer mode, and neither is guaranteed in CI.
func TestDispatch_SymlinkLoop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires special privileges on Windows")
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.Symlink(b, a); err != nil {
		t.Fatalf("symlink a→b: %v", err)
	}
	if err := os.Symlink(a, b); err != nil {
		t.Fatalf("symlink b→a: %v", err)
	}

	err := Run(t.Context(), RunOptions{
		Command: a,
		Cwd:     dir,
	})
	if err == nil {
		t.Fatal("expected loop error, got nil")
	}
	// The exact error varies by OS; any of these message fragments is
	// acceptable evidence that the loop was detected.
	msg := err.Error()
	if !strings.Contains(msg, "too many") &&
		!strings.Contains(msg, "loop") &&
		!strings.Contains(msg, "level") {
		t.Fatalf("expected symlink-loop-ish error, got: %v", err)
	}
}

// TestResolveInterpreter_PermissiveFallback confirms the key portability
// behavior: a literal shebang path that doesn't exist falls back to a
// PATH-lookup on its basename. This is what makes #!/bin/bash work on a
// Windows box where bash.exe lives somewhere else on PATH. We construct a
// fake PATH in a tempdir rather than depending on what the host has
// installed so the test is deterministic everywhere.
func TestResolveInterpreter_PermissiveFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		// exec.LookPath on Windows requires a recognized extension
		// (.exe/.bat/.cmd). Producing one of those without a compiler
		// run is more ceremony than this smoke test deserves; the
		// logic under test is exercised by the Unix run.
		t.Skip("Windows PATH lookup requires an extension-matched binary")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "bash")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake bash: %v", err)
	}
	t.Setenv("PATH", dir)

	// Basename must match the fake we planted on PATH; the directory
	// prefix must not exist so the literal stat fails.
	missingDir := filepath.Join(dir, "definitely-not-here-"+randSuffix())
	resolved, err := resolveInterpreter(filepath.Join(missingDir, "bash"))
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	if resolved != fake {
		t.Fatalf("resolved = %q, want %q", resolved, fake)
	}
}

// TestResolveInterpreter_NonENOENTErrorsSurface guards against silently
// falling back to PATH when stat fails for a reason other than the file
// being missing. With a directory at the shebang path, os.Stat succeeds
// (no fallback needed), but with an EACCES'd file it fails with a non-
// ENOENT error that must be surfaced — otherwise we'd silently resolve a
// different binary off PATH and hide the real problem.
func TestResolveInterpreter_NonENOENTErrorsSurface(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission model")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses dir mode permission checks")
	}
	dir := t.TempDir()
	// Put a candidate interpreter inside an unreadable/untraversable dir.
	inner := filepath.Join(dir, "private")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	interp := filepath.Join(inner, "bash")
	if err := os.WriteFile(interp, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write interpreter: %v", err)
	}
	// Drop search permission on inner so os.Stat(interp) returns EACCES.
	if err := os.Chmod(inner, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(inner, 0o755) })

	_, err := resolveInterpreter(interp)
	if err == nil {
		t.Fatal("expected error for unreadable interpreter, got nil")
	}
	// Must NOT have silently fallen back — the returned path shouldn't
	// be a valid resolution; either way, the error has to surface.
	if !strings.Contains(err.Error(), "permission") {
		t.Fatalf("expected permission-denied error to surface, got: %v", err)
	}
}
