//go:build windows

package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectGitBash(t *testing.T) {
	gitBashPath := detectGitBash()

	// If Git Bash is installed, verify the path exists
	if gitBashPath != "" {
		if _, err := os.Stat(gitBashPath); err != nil {
			t.Errorf("Detected Git Bash path does not exist: %s", gitBashPath)
		}
		t.Logf("Git Bash detected at: %s", gitBashPath)
	} else {
		t.Log("Git Bash not detected (this is OK if not installed)")
	}
}

func TestDetectNativeShellTools(t *testing.T) {
	paths := detectNativeShellTools()

	if len(paths) > 0 {
		t.Logf("Detected %d native shell path(s):", len(paths))
		for i, path := range paths {
			if !dirExists(path) {
				t.Errorf("Path %d does not exist: %s", i, path)
			}
			t.Logf("  [%d] %s", i, path)
		}
	} else {
		t.Log("No native shell tools detected (this is OK if Git Bash/WSL not installed)")
	}
}

func TestExtendEnvWithNativeTools(t *testing.T) {
	// Save original state
	originalPaths := nativeShellPaths

	// Test with no native tools
	nativeShellPaths = []string{}
	env := []string{}
	extended := ExtendEnvWithNativeTools(env)
	if len(extended) != 0 {
		t.Error("Environment should be unchanged when no native tools detected")
	}

	// Test with native tools detected
	nativeShellPaths = []string{`C:\Git\bin`, `C:\Git\usr\bin`}
	env = []string{}
	extended = ExtendEnvWithNativeTools(env)

	hasPath := false
	for _, e := range extended {
		if len(e) > 5 && strings.EqualFold(e[:5], "PATH=") {
			hasPath = true
			pathValue := e[5:]
			t.Logf("Extended PATH: %s", pathValue)

			// Verify native shell paths are prepended
			if !strings.HasPrefix(pathValue, `C:\Git\bin`) {
				t.Error("Native shell paths should be prepended to PATH")
			}
			break
		}
	}
	if !hasPath {
		t.Error("PATH variable should be added when native tools detected")
	}

	// Test with existing PATH
	existingPath := `C:\existing\path`
	envWithPath := []string{"PATH=" + existingPath}
	extended = ExtendEnvWithNativeTools(envWithPath)

	for _, e := range extended {
		if len(e) > 5 && strings.EqualFold(e[:5], "PATH=") {
			pathValue := e[5:]
			t.Logf("Extended PATH with existing: %s", pathValue)

			// Should contain both native paths and existing path
			if !strings.HasPrefix(pathValue, `C:\Git\bin`) {
				t.Error("Native shell paths should be prepended")
			}
			if !strings.HasSuffix(pathValue, existingPath) {
				t.Error("Existing PATH should be preserved at the end")
			}
			break
		}
	}

	// Restore original state
	nativeShellPaths = originalPaths
}

func TestHasNativeShellTools(t *testing.T) {
	// Save original state
	originalPaths := nativeShellPaths

	// Test with no tools
	nativeShellPaths = []string{}
	if HasNativeShellTools() {
		t.Error("HasNativeShellTools should return false when no paths detected")
	}

	// Test with tools
	nativeShellPaths = []string{`C:\Git\bin`}
	if !HasNativeShellTools() {
		t.Error("HasNativeShellTools should return true when paths detected")
	}

	// Restore original state
	nativeShellPaths = originalPaths
}
