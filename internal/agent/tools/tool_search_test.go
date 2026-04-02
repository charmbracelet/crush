package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type toolSearchRegistryStub struct {
	entries []RegistryEntry
}

func (s toolSearchRegistryStub) Search(query string, opts RegistrySearchOptions) []RegistryEntry {
	return SearchRegistryEntries(s.entries, query, opts)
}

func (s toolSearchRegistryStub) Resolve(name string) (RegistryEntry, bool) {
	for _, entry := range s.entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return RegistryEntry{}, false
}

func (s toolSearchRegistryStub) Invoke(context.Context, string, map[string]any, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.NewTextErrorResponse("not implemented"), nil
}

func runToolSearch(t *testing.T, tool fantasy.AgentTool, params ToolSearchParams) []ToolSearchResult {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  ToolSearchToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	var results []ToolSearchResult
	require.NoError(t, json.Unmarshal([]byte(resp.Content), &results))
	return results
}

func TestToolSearchIncludesDeferredByDefault(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{
			Name:        "view",
			Description: "read file",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"file_path": map[string]any{"type": "string"}}, "required": []string{"file_path"}},
			Required:    []string{"file_path"},
			Exposed:     true,
			Metadata:    ToolMetadata{Exposure: ToolExposureDefault},
		},
		{
			Name:        "sourcegraph",
			Description: "search public repositories",
			Parameters:  map[string]any{"type": "object"},
			Metadata:    ToolMetadata{Exposure: ToolExposureDeferred},
		},
	}}

	tool := NewToolSearchTool(registry, nil)
	results := runToolSearch(t, tool, ToolSearchParams{Query: ""})

	require.Len(t, results, 2)
	require.Equal(t, "view", results[0].Name)
	require.Equal(t, "sourcegraph", results[1].Name)
	require.NotNil(t, results[0].Parameters)
	require.Equal(t, []string{"file_path"}, results[0].Required)
}

func TestToolSearchSelectActivatesDeferredTools(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{
			Name:        "sourcegraph",
			Description: "search public repositories",
			Metadata:    ToolMetadata{Exposure: ToolExposureDeferred},
		},
		{
			Name:        "view",
			Description: "read file",
			Metadata:    ToolMetadata{Exposure: ToolExposureDefault},
		},
	}}

	var activated []string
	tool := NewToolSearchTool(registry, func(_ context.Context, toolNames []string) []string {
		activated = append(activated, toolNames...)
		return toolNames
	})

	results := runToolSearch(t, tool, ToolSearchParams{Query: "select:sourcegraph,missing,view"})

	require.Equal(t, []string{"sourcegraph"}, activated)
	require.Len(t, results, 2)
	require.Equal(t, "sourcegraph", results[0].Name)
	require.True(t, results[0].Selected)
	require.True(t, results[0].Activated)
	require.Equal(t, "view", results[1].Name)
	require.True(t, results[1].Selected)
	require.False(t, results[1].Activated)
}
