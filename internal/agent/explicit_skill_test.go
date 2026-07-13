package agent

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

func TestInjectExplicitSkillInvocations(t *testing.T) {
	t.Parallel()

	available := []*skills.Skill{
		{Name: "crush-config", Instructions: "Inspect and preserve the config."},
		{Name: "mcp-setup", Instructions: "Validate the MCP schema first."},
		{Name: "unrelated", Instructions: "Do something else."},
	}
	prompt := "First load and follow the crush-config and mcp-setup skills before editing."

	got, loaded := injectExplicitSkillInvocations(prompt, available)
	require.Equal(t, []string{"crush-config", "mcp-setup"}, loaded)
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

func TestInjectExplicitSkillInvocationsDoesNotInferMCPTasks(t *testing.T) {
	t.Parallel()

	available := []*skills.Skill{
		{Name: "crush-config", Instructions: "Preserve and validate crush.json."},
		{Name: "mcp-setup", Instructions: "Inspect configured MCPs first."},
		{Name: "unrelated", Instructions: "Not relevant."},
	}
	got, loaded := injectExplicitSkillInvocations("Fix the broken MCP configurations on Windows.", available)
	require.Empty(t, loaded)
	require.Equal(t, "Fix the broken MCP configurations on Windows.", got)
}

func TestInjectExplicitSkillInvocationsDoesNotInferHeavyTasks(t *testing.T) {
	t.Parallel()

	available := []*skills.Skill{
		{Name: "execution-routing", Instructions: "Ground, research, delegate, and verify."},
		{Name: "unrelated", Instructions: "Not relevant."},
	}
	prompt := "Investigate the root cause across the repo, fix the multiple failing paths, and verify the implementation autonomously."

	got, loaded := injectExplicitSkillInvocations(prompt, available)
	require.Empty(t, loaded)
	require.Equal(t, prompt, got)
}

func TestSkillTransientContextExcludesUserPrompt(t *testing.T) {
	t.Parallel()

	context := skillTransientContext("fix MCP", "<loaded_skill>instructions</loaded_skill>\n\nfix MCP", []string{"mcp-setup"})
	require.Equal(t, "<loaded_skill>instructions</loaded_skill>", context)
	require.NotContains(t, context, "fix MCP")
}

func TestCoderPromptIsConciseAndSourceDriven(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("templates/coder.md.tpl")
	require.NoError(t, err)
	prompt := string(content)

	require.Less(t, len(prompt), 9_000)
	require.Less(t, strings.Count(prompt, "\n"), 180)

	for _, templateField := range []string{
		"{{.WorkingDir}}",
		"{{if .IsGitRepo}}",
		"{{.Platform}}",
		"{{.Date}}",
		"{{if .GitStatus}}",
		"{{.GitStatus}}",
		"{{if gt (len .Config.LSP) 0}}",
		"{{- if .AvailSkillXML}}",
		"{{.AvailSkillXML}}",
		"{{if .ContextFiles}}",
		"{{range .ContextFiles}}",
		"{{if .GlobalContextFiles}}",
		"{{range .GlobalContextFiles}}",
		"{{.Path}}",
		"{{.Content}}",
	} {
		require.Contains(t, prompt, templateField)
	}

	require.Contains(t, prompt, "full instructions on demand")
	require.NotContains(t, prompt, "MANDATORY activation flow")
	require.NotContains(t, prompt, "before any other tool call")
	for _, duplicatedSection := range []string{
		"<workflow>",
		"<decision_making>",
		"<editing_files>",
		"<whitespace_and_exact_matching>",
		"<task_completion>",
		"<error_handling>",
	} {
		require.NotContains(t, prompt, duplicatedSection)
	}
}
