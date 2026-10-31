package tools

import (
	"reflect"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/schema"
	"github.com/stretchr/testify/require"
)

// toolInputTypes lists every parameter struct that is exposed as a tool input
// schema. New tools must be added here so the description guarantee below
// keeps covering them.
var toolInputTypes = []reflect.Type{
	reflect.TypeFor[BashParams](),
	reflect.TypeFor[CrushInfoParams](),
	reflect.TypeFor[CrushLogsParams](),
	reflect.TypeFor[DiagnosticsParams](),
	reflect.TypeFor[DownloadParams](),
	reflect.TypeFor[EditParams](),
	reflect.TypeFor[FetchParams](),
	reflect.TypeFor[AgenticFetchParams](),
	reflect.TypeFor[WebFetchParams](),
	reflect.TypeFor[WebSearchParams](),
	reflect.TypeFor[GlobParams](),
	reflect.TypeFor[GrepParams](),
	reflect.TypeFor[JobKillParams](),
	reflect.TypeFor[JobOutputParams](),
	reflect.TypeFor[ListMCPResourcesParams](),
	reflect.TypeFor[LSParams](),
	reflect.TypeFor[CallHierarchyParams](),
	reflect.TypeFor[DefinitionParams](),
	reflect.TypeFor[RenameParams](),
	reflect.TypeFor[ReplaceSymbolParams](),
	reflect.TypeFor[LSPRestartParams](),
	reflect.TypeFor[SymbolsParams](),
	reflect.TypeFor[MultiEditParams](),
	reflect.TypeFor[QuestionParams](),
	reflect.TypeFor[ReadMCPResourceParams](),
	reflect.TypeFor[ReferencesParams](),
	reflect.TypeFor[SourcegraphParams](),
	reflect.TypeFor[TodosParams](),
	reflect.TypeFor[ViewParams](),
	reflect.TypeFor[WriteParams](),
}

// TestToolSchemasDescribeEveryParameter guards against emitting a tool schema
// whose properties lack a description. Strict OpenAI-compatible providers
// reject the whole request with "Missing required parameter: description" when
// they see one, which makes every turn fail before any tool can run.
//
// The schema is built through schemaWithItemDescriptions because the generator
// in charm.land/fantasy does not describe array element types. Remove that
// indirection once the generator applies the description tag to item schemas.
func TestToolSchemasDescribeEveryParameter(t *testing.T) {
	t.Parallel()

	for _, typ := range toolInputTypes {
		t.Run(typ.Name(), func(t *testing.T) {
			t.Parallel()

			params := schema.ToParameters(schemaWithItemDescriptions(typ))
			if typ.NumField() == 0 {
				// Tools such as crush_info take no parameters at all.
				return
			}
			require.NotEmpty(t, params, "tool %s exposes no parameters", typ.Name())
			requireDescribed(t, typ.Name(), params)
		})
	}
}

// requireDescribed walks a generated schema and fails on any property, nested
// object property, or array item schema without a non-empty description.
func requireDescribed(t *testing.T, path string, parameters map[string]any) {
	t.Helper()

	for name, raw := range parameters {
		prop, ok := raw.(map[string]any)
		require.Truef(t, ok, "%s.%s: property schema is %T, want map", path, name, raw)

		desc, _ := prop["description"].(string)
		require.NotEmptyf(t, desc, "%s.%s: missing description", path, name)

		if nested, ok := prop["properties"].(map[string]any); ok {
			requireDescribed(t, path+"."+name, nested)
		}
		if items, ok := prop["items"].(map[string]any); ok {
			itemDesc, _ := items["description"].(string)
			require.NotEmptyf(t, itemDesc, "%s.%s[]: missing item description", path, name)
			if nested, ok := items["properties"].(map[string]any); ok {
				requireDescribed(t, path+"."+name+"[]", nested)
			}
		}
	}
}

// TestMultiEditArrayItemsAreDescribed pins the nested case explicitly, since
// multiedit.edits[] is the schema shape that regressed in the field.
func TestMultiEditArrayItemsAreDescribed(t *testing.T) {
	t.Parallel()

	params := schema.ToParameters(schemaWithItemDescriptions(reflect.TypeFor[MultiEditParams]()))
	edits, ok := params["edits"].(map[string]any)
	require.True(t, ok)

	items, ok := edits["items"].(map[string]any)
	require.True(t, ok)

	props, ok := items["properties"].(map[string]any)
	require.True(t, ok)
	for _, field := range []string{"old_string", "new_string", "replace_all"} {
		raw, ok := props[field]
		require.Truef(t, ok, "edits[] is missing %s", field)
		prop, ok := raw.(map[string]any)
		require.True(t, ok)
		require.NotEmptyf(t, prop["description"], "edits[].%s has no description", field)
	}
}

// TestGeneratedSchemaCarriesDescriptions is the target assertion for the
// upstream fix: the schema generator should fall back to the field name when a
// field carries no description tag.
//
// It is skipped because that fallback only exists once it lands in
// charm.land/fantasy, which still emits bare properties as of v0.45.4. Crush
// closes the gap at runtime in internal/agent instead; see
// TestRealToolsDescribeEveryParameter there. Re-enable this test after bumping
// fantasy to a release that fills in the fallback.
func TestGeneratedSchemaCarriesDescriptions(t *testing.T) {
	t.Skip("charm.land/fantasy does not fall back to field names yet")

	type nested struct {
		Value string `json:"value" description:"A nested value"`
	}
	type params struct {
		Count int      `json:"count"`
		Tags  []string `json:"tags"`
		Inner nested   `json:"inner"`
	}

	got := fantasy.ToolInfo{
		Name:       "probe",
		Parameters: schema.ToParameters(schemaWithItemDescriptions(reflect.TypeFor[params]())),
	}

	requireDescribed(t, got.Name, got.Parameters)
	require.Equal(t, "count", got.Parameters["count"].(map[string]any)["description"])
	require.Equal(t, "tags", got.Parameters["tags"].(map[string]any)["description"])
}
