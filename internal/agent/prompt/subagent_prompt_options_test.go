package prompt

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestWithSuppressAvailableSkills verifies the option omits <available_skills>
// from the rendered prompt even though builtin skills exist. Needs a real store
// because the nil-store path never computes AvailSkillXML.
func TestWithSuppressAvailableSkills(t *testing.T) {
	t.Parallel()

	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)

	const tmpl = `{{.AvailSkillXML}}`

	open, err := NewPrompt("t", tmpl)
	require.NoError(t, err)
	got, err := open.Build(context.Background(), "p", "m", store)
	require.NoError(t, err)
	require.Contains(t, got, "<available_skills>", "available skills render by default")

	suppressed, err := NewPrompt("t", tmpl, WithSuppressAvailableSkills(true))
	require.NoError(t, err)
	got, err = suppressed.Build(context.Background(), "p", "m", store)
	require.NoError(t, err)
	require.NotContains(t, got, "<available_skills>", "available skills suppressed by option")
}

// TestWithSubagentBody verifies that WithSubagentBody stores the body string in
// PromptDat.SubagentBody and that the template can render it.
func TestWithSubagentBody(t *testing.T) {
	t.Parallel()

	const body = "You are a specialist agent that does things."

	// Use a template that renders SubagentBody so we can observe the value
	// without needing access to the unexported promptData method.
	p, err := NewPrompt("test", `{{.SubagentBody}}`, WithSubagentBody(body))
	require.NoError(t, err)

	// A nil store makes promptData return a minimal PromptDat (it otherwise
	// needs store.WorkingDir()), which still carries the subagent option fields.
	result, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)
	require.Equal(t, body, result)
}

// TestWithPreloadedSkillsXML verifies that WithPreloadedSkillsXML stores the
// XML string in PromptDat.PreloadedSkillsXML and that the template can render it.
func TestWithPreloadedSkillsXML(t *testing.T) {
	t.Parallel()

	const xml = "<loaded_skill>\n  <name>my-skill</name>\n</loaded_skill>"

	p, err := NewPrompt("test", `{{.PreloadedSkillsXML}}`, WithPreloadedSkillsXML(xml))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)
	require.Equal(t, xml, result)
}

// TestSubagentPromptOptions_BothFieldsInTemplate verifies that both
// SubagentBody and PreloadedSkillsXML are accessible from the template when
// both options are provided.
func TestSubagentPromptOptions_BothFieldsInTemplate(t *testing.T) {
	t.Parallel()

	const (
		body = "Do the specialist thing."
		xml  = "<loaded_skill><name>test-skill</name></loaded_skill>"
	)

	tmpl := `{{.SubagentBody}}|{{.PreloadedSkillsXML}}`
	p, err := NewPrompt("test", tmpl, WithSubagentBody(body), WithPreloadedSkillsXML(xml))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)
	require.Equal(t, body+"|"+xml, result)
}

// TestSubagentPromptOptions_DefaultsToEmpty verifies that SubagentBody and
// PreloadedSkillsXML are empty strings when neither option is provided.
func TestSubagentPromptOptions_DefaultsToEmpty(t *testing.T) {
	t.Parallel()

	tmpl := `body=«{{.SubagentBody}}»xml=«{{.PreloadedSkillsXML}}»`
	p, err := NewPrompt("test", tmpl)
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)
	require.Equal(t, "body=«»xml=«»", result)
}

// TestWithSubagentBody_EmptyString verifies that an empty body string is stored
// and rendered correctly (no panic, no unexpected fallback).
func TestWithSubagentBody_EmptyString(t *testing.T) {
	t.Parallel()

	p, err := NewPrompt("test", `{{.SubagentBody}}`, WithSubagentBody(""))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)
	require.Equal(t, "", result)
}

// TestWithPreloadedSkillsXML_EmptyString verifies that an empty XML string is
// stored and rendered correctly.
func TestWithPreloadedSkillsXML_EmptyString(t *testing.T) {
	t.Parallel()

	p, err := NewPrompt("test", `{{.PreloadedSkillsXML}}`, WithPreloadedSkillsXML(""))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)
	require.Equal(t, "", result)
}

// TestWithAvailableSubagentsXML verifies that WithAvailableSubagentsXML stores
// the XML string in PromptDat.AvailSubagentXML and that the template can
// render it.
func TestWithAvailableSubagentsXML(t *testing.T) {
	t.Parallel()

	const xml = "<available_subagents>\n  <subagent>\n    <name>my-agent</name>\n  </subagent>\n</available_subagents>"

	p, err := NewPrompt("test", `{{.AvailSubagentXML}}`, WithAvailableSubagentsXML(xml))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)
	require.Equal(t, xml, result)
}

// TestWithAvailableSubagentsXML_EmptyString verifies that an empty XML string
// is stored and rendered correctly (default when the option is not provided).
func TestWithAvailableSubagentsXML_EmptyString(t *testing.T) {
	t.Parallel()

	p, err := NewPrompt("test", `{{.AvailSubagentXML}}`, WithAvailableSubagentsXML(""))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", nil)
	require.NoError(t, err)
	require.Equal(t, "", result)
}
