package agent

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

func TestInjectExplicitSkillInvocations(t *testing.T) {
	t.Parallel()

	available := []*skills.Skill{
		{Name: "crush-config", Instructions: "Inspect and preserve the config."},
		{Name: "mcp-schema-first", Instructions: "Validate the MCP schema first."},
		{Name: "unrelated", Instructions: "Do something else."},
	}
	prompt := "First load and follow the crush-config and mcp-schema-first skills before editing."

	got, loaded := injectExplicitSkillInvocations(prompt, available)
	require.Equal(t, []string{"crush-config", "mcp-schema-first"}, loaded)
	require.Contains(t, got, "Inspect and preserve the config.")
	require.Contains(t, got, "Validate the MCP schema first.")
	require.NotContains(t, got, "Do something else.")
	require.True(t, strings.HasSuffix(got, prompt))
}

func TestInjectExplicitSkillInvocationsDoesNotInferFromNameAlone(t *testing.T) {
	t.Parallel()

	prompt := "Is crush-config enabled?"
	got, loaded := injectExplicitSkillInvocations(prompt, []*skills.Skill{
		{Name: "crush-config", Instructions: "Config procedure."},
	})
	require.Equal(t, prompt, got)
	require.Empty(t, loaded)
}
