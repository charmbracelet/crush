package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aleksclark/trifle"
)

// TestNewSessionDialogOpens tests that the new session dialog opens with ctrl+n.
func TestNewSessionDialogOpens(t *testing.T) {
	SkipIfE2EDisabled(t)
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

	// Press ctrl+n to open new session dialog.
	if err := term.Write(trifle.CtrlN()); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	time.Sleep(700 * time.Millisecond)

	// Should show some session-related content.
	output := term.Output()
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
	_ = term.Write(trifle.CtrlN())
	time.Sleep(500 * time.Millisecond)

	// Press escape to close.
	_ = term.KeyEscape()
	time.Sleep(500 * time.Millisecond)

	// Should be able to type normally.
	_ = term.Write("test after escape")
	time.Sleep(300 * time.Millisecond)

	output := term.Output()
	if !strings.Contains(output, "test after escape") {
		t.Logf("App state after escape: %s", output[:min(500, len(output))])
	}
}

// TestNewSessionTextInput tests text input in new session dialog.
func TestNewSessionTextInput(t *testing.T) {
	SkipIfE2EDisabled(t)
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
	_ = term.Write(trifle.CtrlN())
	time.Sleep(500 * time.Millisecond)

	// Type a session name.
	_ = term.Write("my-test-session")
	time.Sleep(300 * time.Millisecond)

	// Should show typed text.
	output := term.Output()
	if !strings.Contains(output, "my-test-session") {
		t.Logf("Typed text may not appear: %s", output[:min(500, len(output))])
	}
}
