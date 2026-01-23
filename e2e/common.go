package e2e

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vttest"
	"github.com/stretchr/testify/require"
)

// update indicates whether to update testdata snapshot files.
var update = flag.Bool("update", false, "update testdata snapshot files")

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
	bin := filepath.Join(testDir, "..", "crush")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return bin
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

// NewTestTerminal creates a new terminal running the crush binary with given args.
func NewTestTerminal(t *testing.T, args []string, cols, rows int) *vttest.Terminal {
	t.Helper()

	term, err := vttest.NewTerminal(t, cols, rows)
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}

	cmd := exec.CommandContext(context.Background(), CrushBinary(), args...)
	if err := term.Start(cmd); err != nil {
		term.Close()
		t.Fatalf("Failed to start crush: %v", err)
	}

	// Give the terminal time to process output for short-lived commands.
	time.Sleep(200 * time.Millisecond)

	return term
}

// NewIsolatedTerminal creates a terminal with isolated config environment.
// Uses the default TestConfigJSON for configuration.
func NewIsolatedTerminal(t *testing.T, cols, rows int) *vttest.Terminal {
	t.Helper()
	return NewIsolatedTerminalWithConfig(t, cols, rows, TestConfigJSON())
}

// NewIsolatedTerminalWithConfig creates a terminal with isolated config environment
// using the provided config JSON string.
func NewIsolatedTerminalWithConfig(t *testing.T, cols, rows int, configJSON string) *vttest.Terminal {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", "crush")
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configFile := filepath.Join(configPath, "crush.json")
	if err := os.WriteFile(configFile, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	term, err := vttest.NewTerminal(t, cols, rows)
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}

	cmd := exec.CommandContext(context.Background(), CrushBinary())
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(tmpDir, "config"),
		"XDG_DATA_HOME="+filepath.Join(tmpDir, "data"),
		"HOME="+tmpDir,
		"USERPROFILE="+tmpDir, // Windows equivalent of HOME
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

// WaitForCondition waits for a condition on the snapshot to be true.
func WaitForCondition(t *testing.T, term *vttest.Terminal, condition func(vttest.Snapshot) bool, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snap := term.Snapshot()
		if condition(snap) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// RequireSnapshot compares the terminal state against a golden JSON snapshot.
// Use -update flag to regenerate snapshots.
func RequireSnapshot(t *testing.T, term *vttest.Terminal, name string) {
	t.Helper()

	snap := term.Snapshot()
	fp := filepath.Join("testdata", t.Name()+"_"+name+".json")

	if *update {
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatalf("Failed to create testdata dir: %v", err)
		}
		f, err := os.Create(fp)
		if err != nil {
			t.Fatalf("Failed to create snapshot file: %v", err)
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(snap); err != nil {
			t.Fatalf("Failed to encode snapshot: %v", err)
		}
		return
	}

	expectedFile, err := os.Open(fp)
	if err != nil {
		t.Fatalf("Failed to read snapshot file %s: %v (run with -update to create)", fp, err)
	}
	defer expectedFile.Close()

	var expected vttest.Snapshot
	if err := json.NewDecoder(expectedFile).Decode(&expected); err != nil {
		t.Fatalf("Failed to decode snapshot: %v", err)
	}

	require.Equal(t, expected, snap, "Snapshot mismatch for %s", name)
}

// RequireTextSnapshot compares just the text content against a golden file.
// This is more lenient than full snapshot comparison - ignores colors/styles.
func RequireTextSnapshot(t *testing.T, term *vttest.Terminal, name string) {
	t.Helper()

	snap := term.Snapshot()
	text := SnapshotText(snap)
	fp := filepath.Join("testdata", t.Name()+"_"+name+".txt")

	if *update {
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatalf("Failed to create testdata dir: %v", err)
		}
		if err := os.WriteFile(fp, []byte(text), 0o644); err != nil {
			t.Fatalf("Failed to write snapshot: %v", err)
		}
		return
	}

	expected, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("Failed to read snapshot file %s: %v (run with -update to create)", fp, err)
	}

	require.Equal(t, string(expected), text, "Text snapshot mismatch for %s", name)
}

// AssertCursorPosition checks the cursor is at the expected position.
func AssertCursorPosition(t *testing.T, snap vttest.Snapshot, x, y int) {
	t.Helper()
	require.Equal(t, x, snap.Cursor.Position.X, "Cursor X position mismatch")
	require.Equal(t, y, snap.Cursor.Position.Y, "Cursor Y position mismatch")
}

// AssertCursorVisible checks the cursor visibility.
func AssertCursorVisible(t *testing.T, snap vttest.Snapshot, visible bool) {
	t.Helper()
	require.Equal(t, visible, snap.Cursor.Visible, "Cursor visibility mismatch")
}

// AssertTitle checks the terminal title.
func AssertTitle(t *testing.T, snap vttest.Snapshot, title string) {
	t.Helper()
	require.Equal(t, title, snap.Title, "Terminal title mismatch")
}

// AssertAltScreen checks if alternate screen is active.
func AssertAltScreen(t *testing.T, snap vttest.Snapshot, active bool) {
	t.Helper()
	require.Equal(t, active, snap.AltScreen, "Alt screen state mismatch")
}

// AssertDimensions checks the terminal dimensions.
func AssertDimensions(t *testing.T, snap vttest.Snapshot, cols, rows int) {
	t.Helper()
	require.Equal(t, cols, snap.Cols, "Terminal columns mismatch")
	require.Equal(t, rows, snap.Rows, "Terminal rows mismatch")
}
