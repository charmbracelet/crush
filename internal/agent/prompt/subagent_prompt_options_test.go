package prompt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWithSubagentBody verifies that WithSubagentBody stores the body string in
// PromptDat.SubagentBody and that the template can render it.
func TestWithSubagentBody(t *testing.T) {
	t.Parallel()

	const body = "You are a specialist agent that does things."

	// Use a template that renders SubagentBody so we can observe the value
	// without needing access to the unexported promptData method.
	p, err := NewPrompt("test", `{{.SubagentBody}}`, WithSubagentBody(body))
	require.NoError(t, err)

	// NewLiteralPrompt path accepts nil store. Template path calls promptData
	// which uses store.WorkingDir(); use a real config store instead.
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
