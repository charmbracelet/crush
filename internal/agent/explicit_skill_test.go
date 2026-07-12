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

func TestInjectExplicitSkillInvocationsRoutesMCPTasks(t *testing.T) {
	t.Parallel()

	available := []*skills.Skill{
		{Name: "crush-config", Instructions: "Preserve and validate crush.json."},
		{Name: "mcp-schema-first", Instructions: "Inspect configured MCPs first."},
		{Name: "unrelated", Instructions: "Not relevant."},
	}
	got, loaded := injectExplicitSkillInvocations("Fix the broken MCP configurations on Windows.", available)
	require.Equal(t, []string{"crush-config", "mcp-schema-first"}, loaded)
	require.Contains(t, got, "Inspect crush_info before editing configuration.")
	require.Contains(t, got, "After a change, call mcp_refresh")
	require.NotContains(t, got, "Preserve and validate crush.json.")
	require.NotContains(t, got, "Inspect configured MCPs first.")
}

func TestInjectExplicitSkillInvocationsRoutesHeavyTasks(t *testing.T) {
	t.Parallel()

	available := []*skills.Skill{
		{Name: "execution-routing", Instructions: "Ground, research, delegate, and verify."},
		{Name: "unrelated", Instructions: "Not relevant."},
	}
	prompt := "Investigate the root cause across the repo, fix the multiple failing paths, and verify the implementation autonomously."

	got, loaded := injectExplicitSkillInvocations(prompt, available)
	require.Equal(t, []string{"execution-routing"}, loaded)
	require.Contains(t, got, "Keep ownership of the user task.")
	require.NotContains(t, got, "Ground, research, delegate, and verify.")
}

func TestSkillTransientContextExcludesUserPrompt(t *testing.T) {
	t.Parallel()

	context := skillTransientContext("fix MCP", "<loaded_skill>instructions</loaded_skill>\n\nfix MCP", []string{"mcp-schema-first"})
	require.Equal(t, "<loaded_skill>instructions</loaded_skill>", context)
	require.NotContains(t, context, "fix MCP")
}
