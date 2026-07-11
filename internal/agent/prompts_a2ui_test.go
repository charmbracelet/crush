package agent

import (
	"strings"
	"testing"
	"text/template"

	"github.com/charmbracelet/crush/internal/agent/prompt"
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
