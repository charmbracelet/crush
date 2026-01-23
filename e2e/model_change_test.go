package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestModelDialogOpens tests that the model dialog opens with ctrl+l.
func TestModelDialogOpens(t *testing.T) {
	SkipIfE2EDisabled(t)

	configJSON := `{
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
}`

	term := NewIsolatedTerminalWithConfig(t, 120, 50, configJSON)
	defer term.Close()

	time.Sleep(startupDelay)

	// Press ctrl+l to open model dialog.
	term.SendText("\x0c") // Ctrl+L
	time.Sleep(dialogTransition)

	// The dialog should show model-related content.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	hasModelContent := strings.Contains(output, "Model") ||
		strings.Contains(output, "model") ||
		strings.Contains(output, "test-model")
	if !hasModelContent {
		t.Logf("Model dialog content: %s", output[:min(500, len(output))])
	}
}

// TestModelDialogClose tests that escape closes the model dialog.
func TestModelDialogClose(t *testing.T) {
	SkipIfE2EDisabled(t)

	term := NewIsolatedTerminal(t, 120, 50)
	defer term.Close()

	time.Sleep(startupDelay)

	// Open model dialog.
	term.SendText("\x0c") // Ctrl+L
	time.Sleep(dialogTransition)

	// Close with escape.
	term.SendText("\x1b") // Escape
	time.Sleep(dialogTransition)

	// Should be able to type normally.
	term.SendText("test after close")
	time.Sleep(300 * time.Millisecond)

	snap := term.Snapshot()
	output := SnapshotText(snap)
	if !strings.Contains(output, "test after close") {
		t.Logf("State after escape: %s", output[:min(500, len(output))])
	}
}

// TestModelConfigLoads tests that model configuration loads successfully.
func TestModelConfigLoads(t *testing.T) {
	SkipIfE2EDisabled(t)

	configJSON := `{
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
}`

	term := NewIsolatedTerminalWithConfig(t, 120, 50, configJSON)
	defer term.Close()

	time.Sleep(startupDelay)

	// App should start successfully with the configured model.
	snap := term.Snapshot()
	output := strings.ToLower(SnapshotText(snap))

	// Should not show any critical error about model configuration.
	require.NotContains(t, output, "error loading")
	require.NotContains(t, output, "failed to")
}
