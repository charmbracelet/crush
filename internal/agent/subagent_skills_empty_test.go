package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// buildDispatchedSubagentPrompt renders the subagent prompt the way the agent
// tool does: the coordinator's active skills are handed over as the available
// skills XML, and the subagent definition decides whether it is shown.
func buildDispatchedSubagentPrompt(t *testing.T, skillNames []string, active []*skills.Skill) string {
	t.Helper()
	sa := newTestSubagent("scoped", skillNames, "Body.")
	p, err := subagentPrompt(sa, active, prompt.WithAvailableSkillsXML(skills.ToPromptXML(active)))
	require.NoError(t, err)
	got, err := p.Build(context.Background(), "p", "m", newPromptTestStore(t))
	require.NoError(t, err)
	return got
}

func TestSubagentPrompt_SkillsExposure(t *testing.T) {
	t.Parallel()

	active := []*skills.Skill{newTestSkill("alpha", false), newTestSkill("beta", false)}

	t.Run("unset skills offers every available skill", func(t *testing.T) {
		t.Parallel()
		got := buildDispatchedSubagentPrompt(t, nil, active)
		require.Contains(t, got, "<available_skills>")
		require.Contains(t, got, "alpha")
		require.Contains(t, got, "beta")
	})

	t.Run("explicit empty skills offers none", func(t *testing.T) {
		t.Parallel()
		got := buildDispatchedSubagentPrompt(t, []string{}, active)
		require.NotContains(t, got, "<available_skills>")
		require.NotContains(t, got, "<loaded_skill>")
		require.NotContains(t, got, "alpha")
		require.NotContains(t, got, "beta")
	})

	t.Run("pinned skill is the only one offered", func(t *testing.T) {
		t.Parallel()
		got := buildDispatchedSubagentPrompt(t, []string{"alpha"}, active)
		require.NotContains(t, got, "<available_skills>")
		require.Contains(t, got, "<loaded_skill>")
		require.Contains(t, got, "alpha")
		require.NotContains(t, got, "beta")
	})
}
