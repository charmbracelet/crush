package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/require"
)

// TestCoderPrompt_RendersAvailableSubagents verifies that wiring
// subagents.ToPromptXML through prompt.WithAvailableSubagentsXML into
// coderPrompt produces a system prompt containing the <available_subagents>
// block, the subagent's name/description, and the matching critical_rules
// delegation instruction — the exact path coordinator.go's NewCoordinator
// wires at construction time.
func TestCoderPrompt_RendersAvailableSubagents(t *testing.T) {
	t.Parallel()

	active := []*subagents.Subagent{
		{Name: "go-test-writer", Description: "Writes Go tests before implementation."},
	}

	p, err := coderPrompt(
		prompt.WithAvailableSubagentsXML(subagents.ToPromptXML(active)),
	)
	require.NoError(t, err)

	systemPrompt, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)

	require.Contains(t, systemPrompt, "<available_subagents>")
	require.Contains(t, systemPrompt, "<name>go-test-writer</name>")
	require.Contains(t, systemPrompt, "<description>Writes Go tests before implementation.</description>")
	require.Contains(t, systemPrompt, "DELEGATE TO MATCHING SUBAGENTS")
}

// TestCoderPrompt_OmitsAvailableSubagentsWhenEmpty verifies that when no
// subagent XML option is supplied (no active subagents), the
// <available_subagents> block is absent from the rendered system prompt.
func TestCoderPrompt_OmitsAvailableSubagentsWhenEmpty(t *testing.T) {
	t.Parallel()

	p, err := coderPrompt()
	require.NoError(t, err)

	systemPrompt, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)

	require.NotContains(t, systemPrompt, "<available_subagents>")
}
