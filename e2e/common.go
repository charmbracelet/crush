package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Timing constants for tests.
const (
	startupDelay     = 3 * time.Second
	dialogTransition = 500 * time.Millisecond
)

// SkipIfE2EDisabled skips the test if E2E_SKIP is set.
func SkipIfE2EDisabled(t *testing.T) {
	t.Helper()
	if os.Getenv("E2E_SKIP") != "" {
		t.Skip("E2E tests disabled via E2E_SKIP env var")
	}
}

// CrushBinary returns the path to the crush binary.
// Checks CRUSH_BINARY env var first, then falls back to ../crush.
func CrushBinary() string {
	if path := os.Getenv("CRUSH_BINARY"); path != "" {
		return path
	}
	// Get the directory of this test file.
	_, file, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(file)
	return filepath.Join(testDir, "..", "crush")
}

// IsWindows returns true if running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// TestConfigJSON returns the config JSON for isolated test environments.
func TestConfigJSON() string {
	return `{
  "providers": {
    "test": {
      "type": "openai-compat",
      "base_url": "http://localhost:9999",
      "api_key": "test-key"
    }
  },
  "models": {
    "large": { "provider": "test", "model": "test-model" },
    "small": { "provider": "test", "model": "test-model" }
  }
}`
}

// IsolationScript returns bash commands to isolate from user config.
func IsolationScript() string {
	return `
export HOME="$TMPDIR/home"
export XDG_CONFIG_HOME="$TMPDIR/config"
export XDG_DATA_HOME="$TMPDIR/data"
mkdir -p "$HOME"
mkdir -p "$XDG_CONFIG_HOME/crush"
mkdir -p "$XDG_DATA_HOME/crush"
`
}
