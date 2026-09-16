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

// TestCoderPrompt_MapHint is the same contract for the project index:
// the map hint must render only when the tool is registered —
// otherwise the model calls a tool it doesn't have.
func TestCoderPrompt_MapHint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	render := func(enabled bool) string {
		store, err := config.Init(dir, "", false)
		require.NoError(t, err)
		store.Config().Options.ProjectIndex = &enabled
		p, err := coderPrompt(prompt.WithWorkingDir(dir))
		require.NoError(t, err)
		built, err := p.Build(t.Context(), "prov", "model", store)
		require.NoError(t, err)
		return built.Text
	}

	require.Contains(t, render(true), "call `map` first")
	require.NotContains(t, render(false), "call `map` first")
}
