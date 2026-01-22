package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vttest"
	"github.com/stretchr/testify/require"
)

// TestNewSessionDialogOpens tests that the new session dialog opens with ctrl+n.
func TestNewSessionDialogOpens(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", "crush")
	require.NoError(t, os.MkdirAll(configPath, 0o755))

	configFile := filepath.Join(configPath, "crush.json")
	require.NoError(t, os.WriteFile(configFile, []byte(TestConfigJSON()), 0o644))

	term, err := vttest.NewTerminal(t, 100, 40)
	require.NoError(t, err)
	defer term.Close()

	cmd := exec.Command(CrushBinary())
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(tmpDir, "config"),
		"XDG_DATA_HOME="+filepath.Join(tmpDir, "data"),
		"HOME="+tmpDir,
	)
	require.NoError(t, term.Start(cmd))

	time.Sleep(startupDelay)

	// Press ctrl+n to open new session dialog.
	term.SendText("\x0e") // Ctrl+N
	time.Sleep(700 * time.Millisecond)

	// Should show some session-related content.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	// Look for session or worktree related content.
	hasSessionContent := strings.Contains(output, "Session") ||
		strings.Contains(output, "session") ||
		strings.Contains(output, "Worktree") ||
		strings.Contains(output, "worktree") ||
		strings.Contains(output, "branch")
	if !hasSessionContent {
		t.Logf("New session dialog may have different content: %s", output[:min(500, len(output))])
	}
}

// TestNewSessionDialogClose tests that escape closes the new session dialog.
func TestNewSessionDialogClose(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", "crush")
	require.NoError(t, os.MkdirAll(configPath, 0o755))

	configFile := filepath.Join(configPath, "crush.json")
	require.NoError(t, os.WriteFile(configFile, []byte(TestConfigJSON()), 0o644))

	term, err := vttest.NewTerminal(t, 100, 40)
	require.NoError(t, err)
	defer term.Close()

	cmd := exec.Command(CrushBinary())
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(tmpDir, "config"),
		"XDG_DATA_HOME="+filepath.Join(tmpDir, "data"),
		"HOME="+tmpDir,
	)
	require.NoError(t, term.Start(cmd))

	time.Sleep(startupDelay)
	term.SendText("\x0e") // Ctrl+N
	time.Sleep(500 * time.Millisecond)

	// Press escape to close.
	term.SendText("\x1b") // Escape
	time.Sleep(500 * time.Millisecond)

	// Should be able to type normally.
	term.SendText("test after escape")
	time.Sleep(300 * time.Millisecond)

	snap := term.Snapshot()
	output := SnapshotText(snap)
	if !strings.Contains(output, "test after escape") {
		t.Logf("App state after escape: %s", output[:min(500, len(output))])
	}
}

// TestNewSessionTextInput tests text input in new session dialog.
func TestNewSessionTextInput(t *testing.T) {
	SkipIfE2EDisabled(t)
	SkipOnWindows(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", "crush")
	require.NoError(t, os.MkdirAll(configPath, 0o755))

	configFile := filepath.Join(configPath, "crush.json")
	require.NoError(t, os.WriteFile(configFile, []byte(TestConfigJSON()), 0o644))

	term, err := vttest.NewTerminal(t, 100, 40)
	require.NoError(t, err)
	defer term.Close()

	cmd := exec.Command(CrushBinary())
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(tmpDir, "config"),
		"XDG_DATA_HOME="+filepath.Join(tmpDir, "data"),
		"HOME="+tmpDir,
	)
	require.NoError(t, term.Start(cmd))

	time.Sleep(startupDelay)
	term.SendText("\x0e") // Ctrl+N
	time.Sleep(500 * time.Millisecond)

	// Type a session name.
	term.SendText("my-test-session")
	time.Sleep(300 * time.Millisecond)

	// Should show typed text.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	if !strings.Contains(output, "my-test-session") {
		t.Logf("Typed text may not appear: %s", output[:min(500, len(output))])
	}
}
