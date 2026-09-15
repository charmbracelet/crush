package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestCoderPrompt_ModeAware is the issue-39 blocker contract: the
// headless prompt must never name the question tool — a model that
// hallucinates a call to a tool it doesn't have burns turns on
// tool-not-found errors.
func TestCoderPrompt_ModeAware(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store, err := config.Init(dir, "", false)
	require.NoError(t, err)

	interactive, err := coderPrompt(
		prompt.WithWorkingDir(dir),
		prompt.WithInteractive(true),
	)
	require.NoError(t, err)
	built, err := interactive.Build(t.Context(), "prov", "model", store)
	require.NoError(t, err)
	require.Contains(t, built.Text, "question tool")
	require.Contains(t, built.Text, "Ask one focused question")

	headless, err := coderPrompt(
		prompt.WithWorkingDir(dir),
		prompt.WithInteractive(false),
	)
	require.NoError(t, err)
	builtH, err := headless.Build(t.Context(), "prov", "model", store)
	require.NoError(t, err)
	require.NotContains(t, builtH.Text, "question tool")
	require.Contains(t, builtH.Text, "cannot ask the user")
	require.Contains(t, builtH.Text, "state it in one line")
}
