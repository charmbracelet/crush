package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestCoderPromptA2UIGate pins the A2UI section's gating: it appears in the
// coder prompt only when the host opts in with prompt.WithA2UI (the chat TUI,
// which renders <a2ui-json> blocks via a2tea). Recorded agent cassettes are
// built without the option, so the gate also keeps them byte-stable.
func TestCoderPromptA2UIGate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg, err := config.Init(dir, "", false)
	require.NoError(t, err)

	off, err := coderPrompt(prompt.WithWorkingDir(dir))
	require.NoError(t, err)
	offText, err := off.Build(t.Context(), "test", "test", cfg)
	require.NoError(t, err)
	require.NotContains(t, offText, "<a2ui>")

	on, err := coderPrompt(prompt.WithWorkingDir(dir), prompt.WithA2UI())
	require.NoError(t, err)
	onText, err := on.Build(t.Context(), "test", "test", cfg)
	require.NoError(t, err)
	require.Contains(t, onText, "<a2ui>")
	require.Contains(t, onText, "<a2ui-json>")
	require.Contains(t, onText, "updateComponents")
}
