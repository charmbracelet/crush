package antigravity

import (
	"strings"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitEffort(t *testing.T) {
	tests := []struct {
		id         string
		wantFamily string
		wantLevel  string
	}{
		{id: "gemini-3.8-flash-high", wantFamily: "gemini-3.8-flash", wantLevel: "high"},
		{id: "gemini-3.8-flash-medium", wantFamily: "gemini-3.8-flash", wantLevel: "medium"},
		{id: "gemini-3.8-flash-low", wantFamily: "gemini-3.8-flash", wantLevel: "low"},
		// `extra-low` is the gateway's name for the minimal tier, and has to be
		// matched before `low` swallows it.
		{id: "gemini-3.5-flash-extra-low", wantFamily: "gemini-3.5-flash", wantLevel: "minimal"},
		// Suffixes that are not efforts.
		{id: "gemini-3.8-flash-tiered", wantFamily: "gemini-3.8-flash-tiered"},
		{id: "gemini-2.5-flash-thinking", wantFamily: "gemini-2.5-flash-thinking"},
		{id: "gemini-3.1-flash-lite", wantFamily: "gemini-3.1-flash-lite"},
		{id: "cld-opus-4-6-thinking", wantFamily: "cld-opus-4-6-thinking"},
		{id: "low", wantFamily: "low"},
	}
	for _, tt := range tests {
		family, level := SplitEffort(tt.id)
		assert.Equalf(t, tt.wantFamily, family, "family of %q", tt.id)
		assert.Equalf(t, tt.wantLevel, level, "level of %q", tt.id)
	}
}

// gatewayCatalog is the shape the subscription really reports: a model per
// reasoning tier, plus models that have no tiers at all.
func gatewayCatalog() []catwalk.Model {
	entry := func(id, name string, canReason bool) catwalk.Model {
		return catwalk.Model{
			ID:                     id,
			Name:                   name,
			ContextWindow:          1048576,
			DefaultMaxTokens:       65536,
			CanReason:              canReason,
			SupportsImages:         true,
			DefaultReasoningEffort: mustLevel(id),
		}
	}
	return []catwalk.Model{
		entry("gemini-3.8-flash-high", "Gemini 3.8 Flash (High)", true),
		entry("gemini-3.8-flash-medium", "Gemini 3.8 Flash (Medium)", true),
		entry("gemini-3.8-flash-low", "Gemini 3.8 Flash (Low)", true),
		entry("gemini-3.8-flash-tiered", "gemini-3.8-flash-tiered", true),
		entry("gemini-3.1-pro-high", "Gemini 3.1 Pro (High)", true),
		entry("gemini-3.1-pro-low", "Gemini 3.1 Pro (Low)", true),
		// The labels do not track the suffixes: extra-low is sold as "(Low)"
		// and low as "(Medium)".
		entry("gemini-3.5-flash-extra-low", "Gemini 3.5 Flash (Low)", true),
		entry("gemini-3.5-flash-low", "Gemini 3.5 Flash (Medium)", true),
		entry("gemini-3.1-flash-lite", "Gemini 3.1 Flash Lite", false),
		entry("cld-sonnet-4-6", "Claude Sonnet 4.6 (Thinking)", true),
		entry("gpt-oss-120b-medium", "GPT-OSS 120B (Medium)", true),
	}
}

func mustLevel(id string) string {
	_, level := SplitEffort(id)
	return level
}

func familyOf(t *testing.T, models []catwalk.Model, id string) catwalk.Model {
	t.Helper()
	for _, m := range models {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("no model %q in %v", id, idsOf(models))
	return catwalk.Model{}
}

func idsOf(models []catwalk.Model) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		out = append(out, m.ID)
	}
	return out
}

// CollapseModels is what keeps the picker readable: one entry per model, with
// the tiers the gateway sells as separate ids moved into reasoning levels.
func TestCollapseModels(t *testing.T) {
	collapsed := CollapseModels(gatewayCatalog())

	require.Equal(t,
		[]string{
			"cld-sonnet-4-6",
			"gemini-3.1-flash-lite",
			"gemini-3.1-pro",
			"gemini-3.5-flash",
			"gemini-3.8-flash",
			"gemini-3.8-flash-tiered",
			"gpt-oss-120b",
		},
		idsOf(collapsed),
	)

	flash := familyOf(t, collapsed, "gemini-3.8-flash")
	assert.Equal(t, "Gemini 3.8 Flash", flash.Name)
	assert.Equal(t, []string{"low", "medium", "high"}, flash.ReasoningLevels)
	// The middle tier, not the most expensive one the account happens to have.
	assert.Equal(t, "medium", flash.DefaultReasoningEffort)
	assert.True(t, flash.CanReason)

	pro := familyOf(t, collapsed, "gemini-3.1-pro")
	assert.Equal(t, []string{"low", "high"}, pro.ReasoningLevels)
	assert.Equal(t, "high", pro.DefaultReasoningEffort)
	assert.Equal(t, "Gemini 3.1 Pro", pro.Name)

	minimal := familyOf(t, collapsed, "gemini-3.5-flash")
	assert.Equal(t, []string{"minimal", "low"}, minimal.ReasoningLevels)
	assert.Equal(t, "low", minimal.DefaultReasoningEffort)
	// Both siblings say something different in parentheses; the family name
	// keeps neither tier's label.
	assert.Equal(t, "Gemini 3.5 Flash", minimal.Name)

	// A single tier still becomes a family, so the id never advertises a level.
	oss := familyOf(t, collapsed, "gpt-oss-120b")
	assert.Equal(t, "GPT-OSS 120B", oss.Name)
	assert.Equal(t, []string{"medium"}, oss.ReasoningLevels)

	// Models without tiers are left exactly as reported, including a
	// parenthetical that is part of the name rather than a level.
	claude := familyOf(t, collapsed, "cld-sonnet-4-6")
	assert.Equal(t, "Claude Sonnet 4.6 (Thinking)", claude.Name)
	assert.Empty(t, claude.ReasoningLevels)
	assert.Empty(t, claude.DefaultReasoningEffort)

	lite := familyOf(t, collapsed, "gemini-3.1-flash-lite")
	assert.False(t, lite.CanReason)
	assert.Empty(t, lite.ReasoningLevels)

	tiered := familyOf(t, collapsed, "gemini-3.8-flash-tiered")
	assert.Equal(t, "gemini-3.8-flash-tiered", tiered.Name)
	assert.Empty(t, tiered.ReasoningLevels)
}

// Configuration load folds catalogs every start, so folding must not change an
// already-folded list.
func TestCollapseModelsIsIdempotent(t *testing.T) {
	once := CollapseModels(gatewayCatalog())
	twice := CollapseModels(once)

	require.Equal(t, idsOf(once), idsOf(twice))
	for _, want := range once {
		got := familyOf(t, twice, want.ID)
		assert.Equal(t, want.Name, got.Name, "name of %s", want.ID)
		assert.Equal(t, want.ReasoningLevels, got.ReasoningLevels, "levels of %s", want.ID)
		assert.Equal(t, want.DefaultReasoningEffort, got.DefaultReasoningEffort, "default of %s", want.ID)
	}
}

// Grouping a catalog that arrived from a JSON object must not depend on map
// iteration order.
func TestCollapseModelsIsStable(t *testing.T) {
	input := gatewayCatalog()
	reversed := make([]catwalk.Model, 0, len(input))
	for i := len(input) - 1; i >= 0; i-- {
		reversed = append(reversed, input[i])
	}
	assert.Equal(t, idsOf(CollapseModels(input)), idsOf(CollapseModels(reversed)))
}

// The gateway's own catalog goes through the same folding, so a signed-in
// account sees one entry per model.
func TestParseModelsCollapsesTiers(t *testing.T) {
	raw := []byte(`{"models":{
		"gemini-3.8-flash-high":{"displayName":"Gemini 3.8 Flash (High)","maxTokens":1048576,"maxOutputTokens":65536,"supportsThinking":true,"supportsImages":true},
		"gemini-3.8-flash-medium":{"displayName":"Gemini 3.8 Flash (Medium)","maxTokens":1048576,"maxOutputTokens":65536,"supportsThinking":true,"supportsImages":true},
		"gemini-3.8-flash-low":{"displayName":"Gemini 3.8 Flash (Low)","maxTokens":1048576,"maxOutputTokens":65536,"supportsThinking":true,"supportsImages":true},
		"chat_20706":{"displayName":"Internal","maxTokens":1024},
		"claude-sonnet-4-6":{"displayName":"Claude Sonnet 4.6 (Thinking)","maxTokens":250000,"maxOutputTokens":64000,"supportsThinking":true}
	}}`)

	models, err := parseModels(raw)
	require.NoError(t, err)
	require.Equal(t, []string{"cld-sonnet-4-6", "gemini-3.8-flash"}, idsOf(models))

	flash := familyOf(t, models, "gemini-3.8-flash")
	assert.Equal(t, "Gemini 3.8 Flash", flash.Name)
	assert.Equal(t, []string{"low", "medium", "high"}, flash.ReasoningLevels)

	// Folding happens after id mangling, so an intercepted name cannot leak
	// back in through the family id.
	for _, m := range models {
		assert.Falsef(t, strings.Contains(m.ID, "claude") || strings.Contains(m.ID, "anthropic"),
			"model id %q would be intercepted by the Gemini provider", m.ID)
	}
}

// The placeholder catalog is written the way the gateway names its models, so
// folding it is what keeps the picker honest before a first fetch.
func TestProviderDefinitionModelsAreFolded(t *testing.T) {
	definition := ProviderDefinition()

	require.NotEmpty(t, definition.Models)
	for _, m := range definition.Models {
		assert.NotEmptyf(t, m.ID, "empty model id in %#v", m)
		_, level := SplitEffort(m.ID)
		assert.Emptyf(t, level, "placeholder %q still names a reasoning tier", m.ID)
		if len(m.ReasoningLevels) > 0 {
			assert.Truef(t, m.CanReason, "%q lists levels but cannot reason", m.ID)
			assert.Contains(t, m.ReasoningLevels, m.DefaultReasoningEffort)
		}
	}

	assert.NotNil(t, familyOf(t, definition.Models, definition.DefaultLargeModelID))
	assert.NotNil(t, familyOf(t, definition.Models, definition.DefaultSmallModelID))
}
