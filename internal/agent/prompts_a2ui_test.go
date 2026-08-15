package agent

import (
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/skills"
	a2tea "github.com/joestump-agent/a2tea"
	"github.com/stretchr/testify/require"
	a2ui "github.com/tmc/a2ui"
)

// renderCoderTemplate executes the embedded coder template directly with the
// given data — no ConfigStore, no filesystem discovery — so the test is
// hermetic and fast. Recorded agent cassettes build the coder prompt without
// WithA2UI; pinning the gate here keeps those cassettes byte-stable.
func renderCoderTemplate(t *testing.T, dat prompt.PromptDat) string {
	t.Helper()
	tpl, err := template.New("coder").Parse(string(coderPromptTmpl))
	require.NoError(t, err)
	var b strings.Builder
	require.NoError(t, tpl.Execute(&b, dat))
	return b.String()
}

func TestCoderPromptA2UIGate(t *testing.T) {
	t.Parallel()

	off := renderCoderTemplate(t, prompt.PromptDat{})
	require.NotContains(t, off, "<a2ui>")

	on := renderCoderTemplate(t, prompt.PromptDat{A2UI: true, A2UIVersion: a2ui.Version})
	require.Contains(t, on, "<a2ui>")
	// The example payload advertises the protocol version the pinned a2ui
	// library actually speaks, not a hardcoded string.
	require.Contains(t, on, `"version":"`+a2ui.Version+`"`)
}

// a2uiPromptSection returns the rendered <a2ui> block with A2UI enabled —
// the configuration real users get, since the coordinator enables A2UI unless
// options.disable_a2ui is set.
func a2uiPromptSection(t *testing.T) string {
	t.Helper()
	full := renderCoderTemplate(t, prompt.PromptDat{A2UI: true, A2UIVersion: a2ui.Version})
	start := strings.Index(full, "<a2ui>")
	end := strings.Index(full, "</a2ui>")
	require.GreaterOrEqual(t, start, 0, "prompt must contain an <a2ui> section")
	require.Greater(t, end, start, "prompt must contain a closing </a2ui>")
	return full[start:end]
}

// TestA2UIPromptNamesRealTools guards the <a2ui> section against telling the
// model to call a tool that does not exist. The section ships enabled by
// default (coordinator.go applies WithA2UI unless disable_a2ui), but the
// recorded TestCoderAgent cassettes build the prompt WITHOUT A2UI — so no
// cassette covers this text and a fabricated tool name would otherwise reach
// users with the suite green.
func TestA2UIPromptNamesRealTools(t *testing.T) {
	t.Parallel()

	section := a2uiPromptSection(t)

	// Every MCP identifier the section mentions must be either a real
	// registered tool or a real parameter of one. Extend these sets when the
	// section starts naming something else.
	knownTools := map[string]bool{
		tools.ReadMCPResourceToolName:  true,
		tools.ListMCPResourcesToolName: true,
	}
	knownParams := map[string]bool{"mcp_name": true}
	for _, name := range regexp.MustCompile(`[a-z][a-z0-9_]{3,}`).FindAllString(section, -1) {
		if !strings.Contains(name, "mcp") || knownParams[name] {
			continue
		}
		require.True(t, knownTools[name],
			"the <a2ui> section names %q, which is not a registered tool", name)
	}

	// read_mcp_resource requires mcp_name; a call example that omits it
	// produces a tool error rather than a surface.
	if strings.Contains(section, tools.ReadMCPResourceToolName) {
		require.Contains(t, section, "mcp_name",
			"the section tells the model to call %s but never mentions its required mcp_name parameter",
			tools.ReadMCPResourceToolName)
	}
}

// TestA2UIPromptInputEditabilityMatchesHost pins the section's claim about
// input components to what the host actually does. Inputs are live: a button
// press harvests FieldValues() and submits them (see A2UISubmissionPrompt and
// internal/ui/model/a2ui_submit_test.go), and the a2ui skill documents them
// under an "input components (editable)" heading. A prompt claiming they are
// read-only talks the model out of a feature that works.
func TestA2UIPromptInputEditabilityMatchesHost(t *testing.T) {
	t.Parallel()

	section := a2uiPromptSection(t)
	require.NotContains(t, strings.ToLower(section), "read-only",
		"input components are editable and submitted; the prompt must not call them read-only")

	skill, err := skills.BuiltinFS().ReadFile("builtin/a2ui/SKILL.md")
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(string(skill)), "input components (editable)",
		"precondition: the a2ui skill documents inputs as editable")
}

// TestA2UIPromptCatalogRenders guards the prompt's component catalog against
// drifting from what the pinned a2tea actually renders: every component the
// <a2ui> section advertises must render as real content, not an
// "[a2tea: ...]" placeholder (which is also what missing/unsupported kinds
// fall back to). If an a2tea bump drops or regresses a component, this fails
// instead of users seeing placeholder junk in chat.
func TestA2UIPromptCatalogRenders(t *testing.T) {
	t.Parallel()

	text := func(id, s string) a2ui.Component {
		return a2ui.Component{ID: id, Text: &a2ui.TextComponent{Text: a2ui.StringLiteral(s)}}
	}

	// One minimal surface per advertised component.
	catalog := map[string][]a2ui.Component{
		"Text":    {{ID: "t", Text: &a2ui.TextComponent{Text: a2ui.StringLiteral("hi"), Variant: a2ui.TextVariantH2}}},
		"Card":    {{ID: "c", Card: &a2ui.CardComponent{Child: "t"}}, text("t", "hi")},
		"Column":  {{ID: "c", Column: &a2ui.ColumnComponent{Children: a2ui.ChildList{IDs: []string{"t"}}}}, text("t", "hi")},
		"Row":     {{ID: "r", Row: &a2ui.RowComponent{Children: a2ui.ChildList{IDs: []string{"t"}}}}, text("t", "hi")},
		"List":    {{ID: "l", List: &a2ui.ListComponent{Children: a2ui.ChildList{IDs: []string{"t"}}}}, text("t", "hi")},
		"Divider": {{ID: "d", Divider: &a2ui.DividerComponent{}}},
		"Button":  {{ID: "b", Button: &a2ui.ButtonComponent{Child: "t"}}, text("t", "OK")},
		"TextField": {{ID: "f", TextField: &a2ui.TextFieldComponent{
			Label: a2ui.StringLiteral("Name"),
		}}},
		"CheckBox": {{ID: "cb", CheckBox: &a2ui.CheckBoxComponent{
			Label: a2ui.StringLiteral("Done"),
			Value: a2ui.BoolLiteral(true),
		}}},
		"ChoicePicker": {{ID: "cp", ChoicePicker: &a2ui.ChoicePickerComponent{
			Options: []a2ui.ChoiceOption{{Value: "a", Label: a2ui.StringLiteral("A")}},
			Value:   a2ui.DynamicStringList{Literal: []string{"a"}},
		}}},
		"Slider": {{ID: "s", Slider: &a2ui.SliderComponent{
			Max:   100,
			Value: a2ui.NumberLiteral(40),
		}}},
		"DateTimeInput": {{ID: "dt", DateTimeInput: &a2ui.DateTimeInputComponent{
			Value: a2ui.StringLiteral("2026-07-11"),
		}}},
	}

	for name, comps := range catalog {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			msgs := []a2ui.ServerMessage{{
				Version:          a2ui.Version,
				UpdateComponents: &a2ui.UpdateComponents{SurfaceID: "s", Components: comps},
			}}
			m, err := a2tea.Render(msgs)
			require.NoError(t, err, "advertised component %s must render", name)
			out := m.View().Content
			require.NotContains(t, out, "[a2tea:",
				"advertised component %s rendered a placeholder: %q", name, out)
			require.NotEmpty(t, strings.TrimSpace(out))
		})
	}
}
