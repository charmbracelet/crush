//go:build windows

package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeHomeForPOSIXShell(t *testing.T) {
	env := []string{"PATH=C:\\Windows", "HOME=C:\\Users\\alice", "TERM=xterm"}
	got := normalizeHomeForPOSIXShell(env)

	want := []string{"PATH=C:\\Windows", "HOME=C:/Users/alice", "TERM=xterm"}
	if len(got) != len(want) {
		t.Fatalf("length changed: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}

	// Original slice must be left untouched -- other holders of the same
	// backing array (e.g. Shell.env) should not see a mutation they didn't
	// ask for.
	if env[1] != "HOME=C:\\Users\\alice" {
		t.Errorf("input slice was mutated: %q", env[1])
	}
}

func TestNormalizeHomeForPOSIXShell_NoHome(t *testing.T) {
	env := []string{"PATH=C:\\Windows", "TERM=xterm"}
	got := normalizeHomeForPOSIXShell(env)
	if len(got) != 2 || got[0] != env[0] || got[1] != env[1] {
		t.Errorf("env without HOME should pass through unchanged, got %v", got)
	}
}

// TestTildeExpansionSurvivesBackslashHome reproduces crush#3389 end to end:
// a hook-style command using `~` must resolve correctly even when $HOME
// contains backslashes, as it does on Windows when spawned from git-bash/MSYS.
func TestTildeExpansionSurvivesBackslashHome(t *testing.T) {
	home := t.TempDir() // a real Windows tempdir, backslash-separated
	marker := filepath.Join(home, "marker.txt")
	if err := os.WriteFile(marker, []byte("found"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "HOME=") {
			env = append(env, e)
		}
	}
	env = append(env, "HOME="+home)
	shell := NewShell(&Options{WorkingDir: t.TempDir(), Env: env})

	stdout, stderr, err := shell.Exec(t.Context(), "cat ~/marker.txt")
	if err != nil {
		t.Fatalf("expected ~ expansion to resolve, got err=%v stderr=%q", err, stderr)
	}
	if strings.TrimSpace(stdout) != "found" {
		t.Fatalf("stdout = %q, want %q", stdout, "found")
	}
}
