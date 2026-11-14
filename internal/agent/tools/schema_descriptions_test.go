package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// probeTool builds a tool over a params type without running anything, so the
// schema it advertises is the one Crush actually sends to the provider.
func probeTool[N any]() fantasy.AgentTool {
	return NewAgentTool("probe", "probe", func(_ context.Context, _ N, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.ToolResponse{}, nil
	})
}

// toolInputTools covers every parameter struct that is exposed as a tool input
// schema. New tools must be added here so the description guarantee below keeps
// covering them.
var toolInputTools = []struct {
	params string
	tool   fantasy.AgentTool
}{
	{"BashParams", probeTool[BashParams]()},
	{"CrushInfoParams", probeTool[CrushInfoParams]()},
	{"CrushLogsParams", probeTool[CrushLogsParams]()},
	{"DiagnosticsParams", probeTool[DiagnosticsParams]()},
	{"DownloadParams", probeTool[DownloadParams]()},
	{"EditParams", probeTool[EditParams]()},
	{"FetchParams", probeTool[FetchParams]()},
	{"AgenticFetchParams", probeTool[AgenticFetchParams]()},
	{"WebFetchParams", probeTool[WebFetchParams]()},
	{"WebSearchParams", probeTool[WebSearchParams]()},
	{"GlobParams", probeTool[GlobParams]()},
	{"GrepParams", probeTool[GrepParams]()},
	{"JobKillParams", probeTool[JobKillParams]()},
	{"JobOutputParams", probeTool[JobOutputParams]()},
	{"ListMCPResourcesParams", probeTool[ListMCPResourcesParams]()},
	{"LSParams", probeTool[LSParams]()},
	{"CallHierarchyParams", probeTool[CallHierarchyParams]()},
	{"DefinitionParams", probeTool[DefinitionParams]()},
	{"RenameParams", probeTool[RenameParams]()},
	{"ReplaceSymbolParams", probeTool[ReplaceSymbolParams]()},
	{"LSPRestartParams", probeTool[LSPRestartParams]()},
	{"SymbolsParams", probeTool[SymbolsParams]()},
	{"MultiEditParams", probeTool[MultiEditParams]()},
	{"QuestionParams", probeTool[QuestionParams]()},
	{"ReadMCPResourceParams", probeTool[ReadMCPResourceParams]()},
	{"ReferencesParams", probeTool[ReferencesParams]()},
	{"SourcegraphParams", probeTool[SourcegraphParams]()},
	{"TodosParams", probeTool[TodosParams]()},
	{"ViewParams", probeTool[ViewParams]()},
	{"WriteParams", probeTool[WriteParams]()},
}

// TestToolSchemasDescribeEveryParameter guards against advertising a tool
// schema whose properties lack a description. Strict OpenAI-compatible
// providers reject the whole request with "Missing required parameter:
// description" when they see one, which makes every turn fail before any tool
// can run.
func TestToolSchemasDescribeEveryParameter(t *testing.T) {
	t.Parallel()

	for _, probe := range toolInputTools {
		t.Run(probe.params, func(t *testing.T) {
			t.Parallel()

			requireDescribed(t, probe.params, probe.tool.Info().Parameters)
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
			itemsPath := path + "." + name + "[]"
			itemDesc, _ := items["description"].(string)
			require.NotEmptyf(t, itemDesc, "%s: missing item description", itemsPath)
			if nested, ok := items["properties"].(map[string]any); ok {
				requireDescribed(t, itemsPath, nested)
			}
		}
	}
}

// TestMultiEditArrayItemsAreDescribed pins the nested case explicitly, since
// multiedit.edits[] is the schema shape that regressed in the field.
func TestMultiEditArrayItemsAreDescribed(t *testing.T) {
	t.Parallel()

	params := probeTool[MultiEditParams]().Info().Parameters

	edits, ok := params["edits"].(map[string]any)
	require.True(t, ok)

	items, ok := edits["items"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "A single edit operation", items["description"])

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

// TestArrayItemsUseItemDescriptionTag checks that arrays of scalars describe
// their elements, which the generator leaves bare.
func TestArrayItemsUseItemDescriptionTag(t *testing.T) {
	t.Parallel()

	params := probeTool[LSParams]().Info().Parameters

	ignore, ok := params["ignore"].(map[string]any)
	require.True(t, ok)

	items, ok := ignore["items"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "A glob pattern to ignore", items["description"])
}

// TestUntaggedParametersFallBackToFieldNames covers params structs whose fields
// carry no description tag at all, which upstream fantasy emits bare.
func TestUntaggedParametersFallBackToFieldNames(t *testing.T) {
	t.Parallel()

	type nested struct {
		Value string `json:"value"`
	}
	type params struct {
		Count int      `json:"count"`
		Tags  []string `json:"tags"`
		Inner nested   `json:"inner"`
	}

	paramsSchema := probeTool[params]().Info().Parameters
	requireDescribed(t, "params", paramsSchema)
	require.Equal(t, "Count", paramsSchema["count"].(map[string]any)["description"])
	require.Equal(t, "Tags", paramsSchema["tags"].(map[string]any)["description"])

	inner, ok := paramsSchema["inner"].(map[string]any)
	require.True(t, ok)
	value, ok := inner["properties"].(map[string]any)["value"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Value", value["description"])
}

// TestDescribeToolsAddsFallbackDescriptions covers tools Crush does not build
// from a params struct, such as MCP and agent delegation tools.
func TestDescribeToolsAddsFallbackDescriptions(t *testing.T) {
	t.Parallel()

	tool := DescribeTools([]fantasy.AgentTool{bareInfoTool{}})[0]
	params := tool.Info().Parameters
	requireDescribed(t, "bare", params)

	tags, ok := params["tags"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tags", tags["description"])

	items, ok := tags["items"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tags item", items["description"])
}

// TestParallelFlagSurvivesDescription checks that describing a tool does not
// lose the parallel flag the agent schedules on.
func TestParallelFlagSurvivesDescription(t *testing.T) {
	t.Parallel()

	tool := NewParallelAgentTool("probe", "probe", func(_ context.Context, _ LSParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.ToolResponse{}, nil
	})

	require.True(t, tool.Info().Parallel)
}

// TestProviderOptionsSurviveDescription checks that the wrapper still forwards
// provider options, which the agent sets on the last tool of a request.
func TestProviderOptionsSurviveDescription(t *testing.T) {
	t.Parallel()

	tool := probeTool[LSParams]()
	want := fantasy.ProviderOptions{"probe": fantasy.ProviderOptionsData(nil)}
	tool.SetProviderOptions(want)

	require.Equal(t, want, tool.ProviderOptions())
}

// bareInfoTool reports parameters with no descriptions at all, the shape an MCP
// server or a hand-written tool can hand to Crush.
type bareInfoTool struct{}

func (bareInfoTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{
		Name: "bare",
		Parameters: map[string]any{
			"count": map[string]any{"type": "integer"},
			"tags":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"inner": map[string]any{
				"type":       "object",
				"properties": map[string]any{"value": map[string]any{"type": "string"}},
			},
		},
	}
}

func (bareInfoTool) Run(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.ToolResponse{}, nil
}

func (bareInfoTool) ProviderOptions() fantasy.ProviderOptions { return nil }

func (bareInfoTool) SetProviderOptions(fantasy.ProviderOptions) {}
