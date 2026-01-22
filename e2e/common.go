package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vttest"
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

// SkipOnWindows skips the test on Windows.
func SkipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
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

// NewTestTerminal creates a new terminal running the crush binary with given args.
func NewTestTerminal(t *testing.T, args []string, cols, rows int) *vttest.Terminal {
	t.Helper()

	term, err := vttest.NewTerminal(t, cols, rows)
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}

	cmd := exec.Command(CrushBinary(), args...)
	if err := term.Start(cmd); err != nil {
		term.Close()
		t.Fatalf("Failed to start crush: %v", err)
	}

	// Give the terminal time to process output for short-lived commands.
	time.Sleep(200 * time.Millisecond)

	return term
}

// NewIsolatedTerminal creates a terminal with isolated config environment.
func NewIsolatedTerminal(t *testing.T, cols, rows int) *vttest.Terminal {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", "crush")
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configFile := filepath.Join(configPath, "crush.json")
	if err := os.WriteFile(configFile, []byte(TestConfigJSON()), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	term, err := vttest.NewTerminal(t, cols, rows)
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}

	cmd := exec.Command(CrushBinary())
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(tmpDir, "config"),
		"XDG_DATA_HOME="+filepath.Join(tmpDir, "data"),
		"HOME="+tmpDir,
	)
	if err := term.Start(cmd); err != nil {
		term.Close()
		t.Fatalf("Failed to start crush: %v", err)
	}

	return term
}

// SnapshotText returns the text content of the terminal snapshot.
func SnapshotText(snap vttest.Snapshot) string {
	var lines []string
	for _, row := range snap.Cells {
		var line strings.Builder
		for _, cell := range row {
			line.WriteString(cell.Content)
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	// Trim trailing empty lines.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// ContainsText checks if the terminal output contains the given text.
func ContainsText(snap vttest.Snapshot, text string) bool {
	return strings.Contains(SnapshotText(snap), text)
}

// WaitForText waits for the terminal to contain the given text.
func WaitForText(t *testing.T, term *vttest.Terminal, text string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snap := term.Snapshot()
		if ContainsText(snap, text) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
