package planmode

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractProposedPlan(t *testing.T) {
	t.Parallel()

	plan, ok := ExtractProposedPlan("before\n<proposed_plan>\n- Step 1\n- Step 2\n</proposed_plan>\nafter")
	require.True(t, ok)
	require.Equal(t, "- Step 1\n- Step 2", plan)
}

func TestExtractProposedPlanMissingTags(t *testing.T) {
	t.Parallel()

	_, ok := ExtractProposedPlan("no plan here")
	require.False(t, ok)
}

func TestBuildExecutionPrompt(t *testing.T) {
	t.Parallel()

	prompt := BuildExecutionPrompt("- Ship it")
	require.Contains(t, prompt, "Execute the approved plan below")
	require.Contains(t, prompt, "<proposed_plan>")
	require.Contains(t, prompt, "- Ship it")
}
