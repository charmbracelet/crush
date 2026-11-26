//go:build windows

package shell

import (
	"os"
	"path/filepath"
	"testing"

	"mvdan.cc/sh/v3/expand"
)

// TestResolveInterpreter_PermissiveFallback_Windows is the Windows-native
// counterpart to the POSIX permissive-fallback test. It proves the one
// behavior that makes `#!/bin/bash` hooks work on a stock Windows box
// with Git for Windows installed: when the literal interpreter path does
// not exist, we fall back to a PATH-lookup on the basename and that
// lookup accepts any executable extension Windows honors (here, `.bat`).
//
// We plant a bash.bat in a tempdir rather than a .exe because producing
// a .exe would require a toolchain step; LookPath on Windows resolves
// PATHEXT extensions, so .bat is just as valid for the lookup codepath.
func TestResolveInterpreter_PermissiveFallback_Windows(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "bash.bat")
	contents := "@echo off\r\nexit /b 0\r\n"
	if err := os.WriteFile(fake, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake bash.bat: %v", err)
	}
	env := expand.ListEnviron("PATH="+dir, "PATHEXT=.BAT;.CMD;.EXE")

	// Literal path must be absent so the stat fails with ENOENT.
	missing := filepath.Join(dir, "definitely-not-here-"+randSuffix(), "bash")
	resolved, err := resolveInterpreter(dir, env, missing)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	if resolved != fake {
		t.Fatalf("resolved = %q, want %q", resolved, fake)
	}
}
