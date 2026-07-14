package agent

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoordinatorDoesNotRewritePromptsWithKeywordRouters(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("coordinator.go")
	require.NoError(t, err)
	source := string(content)

	require.NotContains(t, source, "mcpRoutingContext(")
	require.NotContains(t, source, "injectExplicitSkillInvocations(")
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
