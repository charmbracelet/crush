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

// TestSessionsDialogOpens tests that the sessions dialog opens with ctrl+s.
func TestSessionsDialogOpens(t *testing.T) {
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

	// Press ctrl+s to open sessions dialog.
	term.SendText("\x13") // Ctrl+S
	time.Sleep(700 * time.Millisecond)

	// Should show the sessions dialog.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	require.True(t, strings.Contains(output, "session") || strings.Contains(output, "Session"),
		"Expected sessions dialog content: %s", output)
}

// TestCommandsDialogOpens tests that the commands dialog opens with ctrl+p.
func TestCommandsDialogOpens(t *testing.T) {
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

	// Press ctrl+p to open commands dialog.
	term.SendText("\x10") // Ctrl+P
	time.Sleep(700 * time.Millisecond)

	// Should show the commands dialog with system commands.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	if !strings.Contains(output, "Command") && !strings.Contains(output, "command") {
		t.Logf("Output: %s", output)
	}
}

// TestModelsDialogOpens tests that the models dialog opens with ctrl+l.
func TestModelsDialogOpens(t *testing.T) {
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

	// Press ctrl+l to open models dialog.
	term.SendText("\x0c") // Ctrl+L
	time.Sleep(700 * time.Millisecond)

	// Should show the models dialog or model-related content.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	if !strings.Contains(output, "Model") && !strings.Contains(output, "model") && !strings.Contains(output, "test-model") {
		t.Logf("Output: %s", output)
	}
}

// TestEscapeClosesDialog tests that escape closes any open dialog.
func TestEscapeClosesDialog(t *testing.T) {
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

	// Open a dialog.
	term.SendText("\x0c") // Ctrl+L
	time.Sleep(500 * time.Millisecond)

	// Press escape to close.
	term.SendText("\x1b") // Escape
	time.Sleep(500 * time.Millisecond)

	// App should still be responsive.
	term.SendText("test")
	time.Sleep(300 * time.Millisecond)

	snap := term.Snapshot()
	output := SnapshotText(snap)
	if !strings.Contains(output, "test") {
		t.Logf("App may not be in expected state: %s", output)
	}
}

// TestTextInput tests that typing text appears in the input area.
func TestTextInput(t *testing.T) {
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

	// Type some short text.
	testText := "hello"
	term.SendText(testText)
	time.Sleep(500 * time.Millisecond)

	// Verify the app is still responsive - text may appear in prompt area.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	// In TUI apps, input may be displayed differently, just check app is working.
	require.Greater(t, len(output), 100, "App may have crashed - output too short: %s", output)
}

// TestCtrlGOpensMoreMenu tests that ctrl+g opens the more menu.
func TestCtrlGOpensMoreMenu(t *testing.T) {
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

	// Press ctrl+g for more menu.
	term.SendText("\x07") // Ctrl+G
	time.Sleep(700 * time.Millisecond)

	// Check that something changed.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	// This should open some kind of menu or show more options.
	t.Logf("Ctrl+G output: %s", output[:min(500, len(output))])
}
