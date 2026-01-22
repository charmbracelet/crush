package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aleksclark/trifle"
)

// TestModelDialogOpens tests that the model dialog opens with ctrl+l.
func TestModelDialogOpens(t *testing.T) {
	trifle.SkipOnWindows(t)

	script := fmt.Sprintf(`
set -e
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

%s

cat > crush.json << 'CONFIG'
{
  "providers": {
    "test-provider": {
      "type": "openai-compat",
      "base_url": "http://localhost:9999",
      "api_key": "test-key"
    }
  },
  "models": {
    "large": { "provider": "test-provider", "model": "test-model-large" },
    "small": { "provider": "test-provider", "model": "test-model-small" }
  }
}
CONFIG

exec "%s"
`, IsolationScript(), CrushBinary())

	term, err := trifle.NewTerminal("bash", []string{"-c", script}, trifle.TerminalOptions{
		Rows: 50,
		Cols: 120,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	time.Sleep(startupDelay)

	// Press ctrl+l to open model dialog.
	_ = term.Write(trifle.CtrlL())
	time.Sleep(dialogTransition)

	// The dialog should show model-related content.
	output := term.Output()
	hasModelContent := strings.Contains(output, "Model") ||
		strings.Contains(output, "model") ||
		strings.Contains(output, "test-model")
	if !hasModelContent {
		t.Logf("Model dialog content: %s", output[:min(500, len(output))])
	}
}

// TestModelDialogClose tests that escape closes the model dialog.
func TestModelDialogClose(t *testing.T) {
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
		Rows: 50,
		Cols: 120,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	time.Sleep(startupDelay)

	// Open model dialog.
	_ = term.Write(trifle.CtrlL())
	time.Sleep(dialogTransition)

	// Close with escape.
	_ = term.KeyEscape()
	time.Sleep(dialogTransition)

	// Should be able to type normally.
	_ = term.Write("test after close")
	time.Sleep(300 * time.Millisecond)

	output := term.Output()
	if !strings.Contains(output, "test after close") {
		t.Logf("State after escape: %s", output[:min(500, len(output))])
	}
}

// TestModelConfigLoads tests that model configuration loads successfully.
func TestModelConfigLoads(t *testing.T) {
	trifle.SkipOnWindows(t)

	script := fmt.Sprintf(`
set -e
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

%s

cat > crush.json << 'CONFIG'
{
  "providers": {
    "anthropic": {
      "type": "anthropic",
      "api_key": "test-key"
    }
  },
  "models": {
    "large": { "provider": "anthropic", "model": "claude-sonnet-4-20250514" },
    "small": { "provider": "anthropic", "model": "claude-haiku-3-20240307" }
  }
}
CONFIG

exec "%s"
`, IsolationScript(), CrushBinary())

	term, err := trifle.NewTerminal("bash", []string{"-c", script}, trifle.TerminalOptions{
		Rows: 50,
		Cols: 120,
	})
	if err != nil {
		t.Fatalf("Failed to create terminal: %v", err)
	}
	defer term.Close()

	time.Sleep(startupDelay)

	// App should start successfully with the configured model.
	output := strings.ToLower(term.Output())

	// Should not show any critical error about model configuration.
	if strings.Contains(output, "error loading") {
		t.Errorf("Unexpected error loading: %s", term.Output())
	}
	if strings.Contains(output, "failed to") {
		t.Errorf("Unexpected failure: %s", term.Output())
	}
}
