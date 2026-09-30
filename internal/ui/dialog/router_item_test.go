package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestRouterItem_RenderShowsLabelAndValue(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	item := NewRouterItem(&s, routerFieldSpecs[0], "false")
	rendered := item.Render(40)
	require.Contains(t, rendered, "Enabled")
	require.Contains(t, rendered, "false")
}

func TestRouterItem_StartEditSeedsInputWithCurrentValue(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	spec := routerFieldSpec{key: "base_url", label: "Base URL", kind: routerFieldText}
	item := NewRouterItem(&s, spec, "http://old")
	item.SetFocused(true)
	item.StartEdit()
	require.True(t, item.Editing())
	require.Equal(t, "http://old", item.InputValue())
}

func TestRouterItem_HandleInputAppendsCharacters(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	spec := routerFieldSpec{key: "model", label: "Model", kind: routerFieldText}
	item := NewRouterItem(&s, spec, "")
	item.SetFocused(true)
	item.StartEdit()
	item.HandleInput(tea.KeyPressMsg{Text: "x", Code: 'x'})
	item.HandleInput(tea.KeyPressMsg{Text: "y", Code: 'y'})
	require.Equal(t, "xy", item.InputValue())
}

func TestRouterItem_CancelEditLeavesValueUnchanged(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	spec := routerFieldSpec{key: "model", label: "Model", kind: routerFieldText}
	item := NewRouterItem(&s, spec, "original")
	item.SetFocused(true)
	item.StartEdit()
	item.HandleInput(tea.KeyPressMsg{Text: "z", Code: 'z'})
	item.CancelEdit()
	require.False(t, item.Editing())
	require.Equal(t, "original", item.Value())
}

func TestRouterItem_SetValueExitsEditMode(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	spec := routerFieldSpec{key: "model", label: "Model", kind: routerFieldText}
	item := NewRouterItem(&s, spec, "original")
	item.SetFocused(true)
	item.StartEdit()
	item.SetValue("committed")
	require.False(t, item.Editing())
	require.Equal(t, "committed", item.Value())
}

func TestRouterItem_SetFocusedFalseCancelsMidEdit(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	spec := routerFieldSpec{key: "model", label: "Model", kind: routerFieldText}
	item := NewRouterItem(&s, spec, "original")
	item.SetFocused(true)
	item.StartEdit()
	item.HandleInput(tea.KeyPressMsg{Text: "z", Code: 'z'})
	item.SetFocused(false)
	require.False(t, item.Editing())
	require.Equal(t, "original", item.Value())
	require.Nil(t, item.Cursor(), "input should be blurred (no cursor) after focus is lost mid-edit")
}

// TestRouterItem_RenderMasksAPIKey pins finding 4 from the final
// whole-branch review: the api_key field's non-editing render must never
// show the raw secret (so opening the dialog while screen-sharing doesn't
// leak it), but should still reveal its last 4 characters so the user can
// confirm which key is configured.
func TestRouterItem_RenderMasksAPIKey(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	spec := routerFieldSpec{key: "api_key", label: "API Key", kind: routerFieldText}
	item := NewRouterItem(&s, spec, "sk-super-secret-1234")
	rendered := item.Render(60)

	require.NotContains(t, rendered, "sk-super-secret-1234")
	require.Contains(t, rendered, "1234", "the last 4 characters must still be visible")
	require.Contains(t, rendered, "•")
}

// TestRouterItem_RenderDoesNotMaskOtherFields pins the scope of the mask:
// only the api_key field is affected, every other field renders its value
// verbatim as before.
func TestRouterItem_RenderDoesNotMaskOtherFields(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	spec := routerFieldSpec{key: "base_url", label: "Base URL", kind: routerFieldText}
	item := NewRouterItem(&s, spec, "http://example.invalid")
	rendered := item.Render(60)

	require.Contains(t, rendered, "http://example.invalid")
	require.NotContains(t, rendered, "•")
}

func TestRouterFieldSpecs_CoversAllSevenSettings(t *testing.T) {
	t.Parallel()

	require.Len(t, routerFieldSpecs, 10)
	keys := make(map[string]bool, len(routerFieldSpecs))
	for _, spec := range routerFieldSpecs {
		keys[spec.key] = true
	}
	for _, want := range []string{
		"enabled", "provider", "base_url", "api_key", "model", "confidence_threshold",
		"timeout_ms", "model_pool", "min_model_confidence", "apply_subagents",
	} {
		require.True(t, keys[want], "missing field spec for %q", want)
	}
}

func TestRouterFieldSpecs_IncludesModelPool(t *testing.T) {
	t.Parallel()

	found := false
	for _, spec := range routerFieldSpecs {
		if spec.key == "model_pool" {
			found = true
			require.Equal(t, routerFieldList, spec.kind)
		}
	}
	require.True(t, found, "routerFieldSpecs must include a model_pool entry")
}
