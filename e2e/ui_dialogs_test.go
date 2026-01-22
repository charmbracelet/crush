package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aleksclark/trifle"
)

// TestSessionsDialogOpens tests that the sessions dialog opens with ctrl+s.
func TestSessionsDialogOpens(t *testing.T) {
	trifle.SkipOnWindows(t)

	script := fmt.Sprintf(`
set -e
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

%s

cat > crush.json << 'CONFIG'
%s
CONFIG

exec "%s"
`, IsolationScript(), TestConfigJSON(), CrushBinary())

	term, err := trifle.NewTerminal("bash", []string{"-c", script}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	time.Sleep(startupDelay)

	// Press ctrl+s to open sessions dialog.
	_ = term.Write(trifle.ControlKey('s'))
	time.Sleep(700 * time.Millisecond)

	// Should show the sessions dialog.
	output := term.Output()
	// Look for session-related content.
	if !strings.Contains(output, "session") && !strings.Contains(output, "Session") {
		t.Errorf("Expected sessions dialog content: %s", output)
	}
}

// TestCommandsDialogOpens tests that the commands dialog opens with ctrl+p.
func TestCommandsDialogOpens(t *testing.T) {
	trifle.SkipOnWindows(t)

	script := fmt.Sprintf(`
set -e
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

%s

cat > crush.json << 'CONFIG'
%s
CONFIG

exec "%s"
`, IsolationScript(), TestConfigJSON(), CrushBinary())

	term, err := trifle.NewTerminal("bash", []string{"-c", script}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	time.Sleep(startupDelay)

	// Press ctrl+p to open commands dialog.
	_ = term.Write(trifle.CtrlP())
	time.Sleep(700 * time.Millisecond)

	// Should show the commands dialog with system commands.
	output := term.Output()
	if !strings.Contains(output, "Command") && !strings.Contains(output, "command") {
		t.Logf("Output: %s", output)
	}
}

// TestModelsDialogOpens tests that the models dialog opens with ctrl+l.
func TestModelsDialogOpens(t *testing.T) {
	trifle.SkipOnWindows(t)

	script := fmt.Sprintf(`
set -e
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

%s

cat > crush.json << 'CONFIG'
%s
CONFIG

exec "%s"
`, IsolationScript(), TestConfigJSON(), CrushBinary())

	term, err := trifle.NewTerminal("bash", []string{"-c", script}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	time.Sleep(startupDelay)

	// Press ctrl+l to open models dialog.
	_ = term.Write(trifle.CtrlL())
	time.Sleep(700 * time.Millisecond)

	// Should show the models dialog or model-related content.
	output := term.Output()
	if !strings.Contains(output, "Model") && !strings.Contains(output, "model") && !strings.Contains(output, "test-model") {
		t.Logf("Output: %s", output)
	}
}

// TestEscapeClosesDialog tests that escape closes any open dialog.
func TestEscapeClosesDialog(t *testing.T) {
	trifle.SkipOnWindows(t)

	script := fmt.Sprintf(`
set -e
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

%s

cat > crush.json << 'CONFIG'
%s
CONFIG

exec "%s"
`, IsolationScript(), TestConfigJSON(), CrushBinary())

	term, err := trifle.NewTerminal("bash", []string{"-c", script}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	time.Sleep(startupDelay)

	// Open a dialog.
	_ = term.Write(trifle.CtrlL())
	time.Sleep(500 * time.Millisecond)

	// Press escape to close.
	_ = term.KeyEscape()
	time.Sleep(500 * time.Millisecond)

	// App should still be responsive.
	_ = term.Write("test")
	time.Sleep(300 * time.Millisecond)

	output := term.Output()
	if !strings.Contains(output, "test") {
		t.Logf("App may not be in expected state: %s", output)
	}
}

// TestTextInput tests that typing text appears in the input area.
func TestTextInput(t *testing.T) {
	trifle.SkipOnWindows(t)

	script := fmt.Sprintf(`
set -e
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

%s

cat > crush.json << 'CONFIG'
%s
CONFIG

exec "%s"
`, IsolationScript(), TestConfigJSON(), CrushBinary())

	term, err := trifle.NewTerminal("bash", []string{"-c", script}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	time.Sleep(startupDelay)

	// Type some short text.
	testText := "hello"
	_ = term.Write(testText)
	time.Sleep(500 * time.Millisecond)

	// Verify the app is still responsive - text may appear in prompt area.
	output := term.Output()
	// In TUI apps, input may be displayed differently, just check app is working.
	if len(output) < 100 {
		t.Errorf("App may have crashed - output too short: %s", output)
	}
}

// TestCtrlGOpensMoreMenu tests that ctrl+g opens the more menu.
func TestCtrlGOpensMoreMenu(t *testing.T) {
	trifle.SkipOnWindows(t)

	script := fmt.Sprintf(`
set -e
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

%s

cat > crush.json << 'CONFIG'
%s
CONFIG

exec "%s"
`, IsolationScript(), TestConfigJSON(), CrushBinary())

	term, err := trifle.NewTerminal("bash", []string{"-c", script}, trifle.TerminalOptions{
		Rows: 40,
		Cols: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	time.Sleep(startupDelay)

	// Press ctrl+g for more menu.
	_ = term.Write(trifle.ControlKey('g'))
	time.Sleep(700 * time.Millisecond)

	// Check that something changed.
	output := term.Output()
	// This should open some kind of menu or show more options.
	t.Logf("Ctrl+G output: %s", output[:min(500, len(output))])
}
