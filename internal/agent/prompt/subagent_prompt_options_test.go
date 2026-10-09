package prompt

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// newTestStore returns a config store for prompt-rendering tests. It builds
// the store directly instead of calling config.Init, which reads the host's
// HOME/XDG locations: a developer with a populated ~/.config/crush would
// otherwise have their own context files and skill paths rendered into the
// prompt under test.
func newTestStore(t *testing.T) *config.ConfigStore {
	t.Helper()
	return config.NewTestStoreWithWorkingDir(&config.Config{Options: &config.Options{}}, t.TempDir())
}

// TestWithSuppressAvailableSkills verifies the option omits <available_skills>
// from the rendered prompt even though builtin skills exist.
func TestWithSuppressAvailableSkills(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

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

	store := newTestStore(t)

	// Use a template that renders SubagentBody so we can observe the value
	// without needing access to the unexported promptData method.
	p, err := NewPrompt("test", `{{.SubagentBody}}`, WithSubagentBody(body))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", store)
	require.NoError(t, err)
	require.Equal(t, body, result)
}

// TestWithPreloadedSkillsXML verifies that WithPreloadedSkillsXML stores the
// XML string in PromptDat.PreloadedSkillsXML and that the template can render it.
func TestWithPreloadedSkillsXML(t *testing.T) {
	t.Parallel()

	const xml = "<loaded_skill>\n  <name>my-skill</name>\n</loaded_skill>"

	store := newTestStore(t)

	p, err := NewPrompt("test", `{{.PreloadedSkillsXML}}`, WithPreloadedSkillsXML(xml))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", store)
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

	store := newTestStore(t)

	p, err := NewPrompt("test", tmpl, WithSubagentBody(body), WithPreloadedSkillsXML(xml))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", store)
	require.NoError(t, err)
	require.Equal(t, body+"|"+xml, result)
}

// TestSubagentPromptOptions_DefaultsToEmpty verifies that SubagentBody and
// PreloadedSkillsXML are empty strings when neither option is provided.
func TestSubagentPromptOptions_DefaultsToEmpty(t *testing.T) {
	t.Parallel()

	tmpl := `body=«{{.SubagentBody}}»xml=«{{.PreloadedSkillsXML}}»`

	store := newTestStore(t)

	p, err := NewPrompt("test", tmpl)
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", store)
	require.NoError(t, err)
	require.Equal(t, "body=«»xml=«»", result)
}

// TestWithSubagentBody_EmptyString verifies that an empty body string is stored
// and rendered correctly (no panic, no unexpected fallback).
func TestWithSubagentBody_EmptyString(t *testing.T) {
	t.Parallel()

	p, err := NewPrompt("test", `{{.SubagentBody}}`, WithSubagentBody(""))
	require.NoError(t, err)

	store := newTestStore(t)

	result, err := p.Build(context.Background(), "test-provider", "test-model", store)
	require.NoError(t, err)
	require.Equal(t, "", result)
}

// TestWithPreloadedSkillsXML_EmptyString verifies that an empty XML string is
// stored and rendered correctly.
func TestWithPreloadedSkillsXML_EmptyString(t *testing.T) {
	t.Parallel()

	p, err := NewPrompt("test", `{{.PreloadedSkillsXML}}`, WithPreloadedSkillsXML(""))
	require.NoError(t, err)

	store := newTestStore(t)

	result, err := p.Build(context.Background(), "test-provider", "test-model", store)
	require.NoError(t, err)
	require.Equal(t, "", result)
}

// TestWithAvailableSubagentsXML verifies that WithAvailableSubagentsXML stores
// the XML string in PromptDat.AvailSubagentXML and that the template can
// render it.
func TestWithAvailableSubagentsXML(t *testing.T) {
	t.Parallel()

	const xml = "<available_subagents>\n  <subagent>\n    <name>my-agent</name>\n  </subagent>\n</available_subagents>"

	store := newTestStore(t)

	p, err := NewPrompt("test", `{{.AvailSubagentXML}}`, WithAvailableSubagentsXML(xml))
	require.NoError(t, err)

	result, err := p.Build(context.Background(), "test-provider", "test-model", store)
	require.NoError(t, err)
	require.Equal(t, xml, result)
}

// TestWithAvailableSubagentsXML_EmptyString verifies that an empty XML string
// is stored and rendered correctly (default when the option is not provided).
func TestWithAvailableSubagentsXML_EmptyString(t *testing.T) {
	t.Parallel()

	p, err := NewPrompt("test", `{{.AvailSubagentXML}}`, WithAvailableSubagentsXML(""))
	require.NoError(t, err)

	store := newTestStore(t)

	result, err := p.Build(context.Background(), "test-provider", "test-model", store)
	require.NoError(t, err)
	require.Equal(t, "", result)
}

// TestWithAvailableSkillsXML verifies the caller-supplied block replaces the
// discovery walk's output.
func TestWithAvailableSkillsXML(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	const tmpl = `{{.AvailSkillXML}}`
	const xml = "<available_skills>\n  <skill><name>supplied</name></skill>\n</available_skills>"

	p, err := NewPrompt("t", tmpl, WithAvailableSkillsXML(xml))
	require.NoError(t, err)
	got, err := p.Build(context.Background(), "p", "m", store)
	require.NoError(t, err)
	require.Equal(t, xml, got, "supplied block must be used verbatim instead of discovery")
}

// TestWithAvailableSkillsXML_EmptyStringSkipsDiscovery verifies that an
// explicitly empty block is honored rather than falling back to the discovery
// walk — the distinction availSkillXMLSet exists to preserve.
func TestWithAvailableSkillsXML_EmptyStringSkipsDiscovery(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	const tmpl = `{{.AvailSkillXML}}`

	open, err := NewPrompt("t", tmpl)
	require.NoError(t, err)
	got, err := open.Build(context.Background(), "p", "m", store)
	require.NoError(t, err)
	require.Contains(t, got, "<available_skills>", "discovery renders builtins by default")

	empty, err := NewPrompt("t", tmpl, WithAvailableSkillsXML(""))
	require.NoError(t, err)
	got, err = empty.Build(context.Background(), "p", "m", store)
	require.NoError(t, err)
	require.Empty(t, got, "an explicitly empty block must not fall back to discovery")
}

// TestWithSuppressAvailableSkills_BeatsSuppliedXML verifies suppression wins
// over a supplied block, so a subagent pinning skills cannot be handed the
// broad discovery list through the other option.
func TestWithSuppressAvailableSkills_BeatsSuppliedXML(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	p, err := NewPrompt("t", `{{.AvailSkillXML}}`,
		WithAvailableSkillsXML("<available_skills><skill><name>leak</name></skill></available_skills>"),
		WithSuppressAvailableSkills(true),
	)
	require.NoError(t, err)
	got, err := p.Build(context.Background(), "p", "m", store)
	require.NoError(t, err)
	require.Empty(t, got, "suppression must win over a supplied skills block")
}
