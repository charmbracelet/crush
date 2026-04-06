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

func runToolSearchResponse(t *testing.T, tool fantasy.AgentTool, params ToolSearchParams) ToolSearchResponse {
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

	var response ToolSearchResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Content), &response))
	return response
}

func runToolSearch(t *testing.T, tool fantasy.AgentTool, params ToolSearchParams) []ToolSearchResult {
	t.Helper()
	response := runToolSearchResponse(t, tool, params)
	return response.Results
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
	response := runToolSearchResponse(t, tool, ToolSearchParams{Query: ""})
	results := response.Results

	require.Len(t, results, 2)
	require.Equal(t, "view", results[0].Name)
	require.Equal(t, "sourcegraph", results[1].Name)
	require.NotNil(t, results[0].Parameters)
	require.Equal(t, []string{"file_path"}, results[0].Required)
	require.Equal(t, []string{"view", "sourcegraph"}, response.Matches)
	require.Equal(t, 1, response.TotalDeferred)
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
			Exposed:     true,
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

func TestToolSearchSelectSkipsUnexposedNonDeferredTools(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{
			Name:        "view",
			Description: "read file",
			Exposed:     true,
			Metadata:    ToolMetadata{Exposure: ToolExposureDefault},
		},
		{
			Name:        "secret_write",
			Description: "internal tool",
			Exposed:     false,
			Metadata:    ToolMetadata{Exposure: ToolExposureDefault},
		},
	}}

	tool := NewToolSearchTool(registry, nil)
	results := runToolSearch(t, tool, ToolSearchParams{Query: "select:view,secret_write"})

	require.Len(t, results, 1)
	require.Equal(t, "view", results[0].Name)
	require.True(t, results[0].Selected)
}

func TestToolSearchExactNameActivatesDeferredTool(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{
			Name:        "sourcegraph",
			Description: "search public repositories",
			Metadata:    ToolMetadata{Exposure: ToolExposureDeferred},
		},
	}}

	var activated []string
	tool := NewToolSearchTool(registry, func(_ context.Context, toolNames []string) []string {
		activated = append(activated, toolNames...)
		return toolNames
	})

	results := runToolSearch(t, tool, ToolSearchParams{Query: "sourcegraph"})

	require.Equal(t, []string{"sourcegraph"}, activated)
	require.Len(t, results, 1)
	require.Equal(t, "sourcegraph", results[0].Name)
	require.True(t, results[0].Selected)
	require.True(t, results[0].Activated)
}

func TestToolSearchKeywordQueryActivatesDeferredMatches(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{
			Name:        "view",
			Description: "read file",
			Exposed:     true,
			Metadata:    ToolMetadata{Exposure: ToolExposureDefault},
		},
		{
			Name:        "sourcegraph",
			Description: "search public repositories",
			Exposed:     false,
			Metadata:    ToolMetadata{Exposure: ToolExposureDeferred},
		},
	}}

	var activated []string
	tool := NewToolSearchTool(registry, func(_ context.Context, toolNames []string) []string {
		activated = append(activated, toolNames...)
		return toolNames
	})

	results := runToolSearch(t, tool, ToolSearchParams{Query: "public repositories"})

	require.Equal(t, []string{"sourcegraph"}, activated)
	require.Len(t, results, 1)
	require.Equal(t, "sourcegraph", results[0].Name)
	require.True(t, results[0].Activated)
	require.False(t, results[0].Selected)
}

func TestToolSearchUsesMaxResultsAlias(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{Name: "view", Description: "read files", Exposed: true, Metadata: ToolMetadata{Exposure: ToolExposureDefault}},
		{Name: "grep", Description: "search files", Exposed: true, Metadata: ToolMetadata{Exposure: ToolExposureDefault}},
		{Name: "glob", Description: "find files", Exposed: true, Metadata: ToolMetadata{Exposure: ToolExposureDefault}},
	}}

	tool := NewToolSearchTool(registry, nil)
	results := runToolSearch(t, tool, ToolSearchParams{Query: "files", MaxResults: 2})

	require.Len(t, results, 2)
}

func TestToolSearchKeywordRankingPrefersHintAndTags(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{
			Name:        "sourcegraph",
			Description: "search repositories",
			Metadata: ToolMetadata{
				Exposure:   ToolExposureDeferred,
				SearchHint: "search public repositories",
				SearchTags: []string{"code-search", "network"},
			},
		},
		{
			Name:        "view",
			Description: "read local files",
			Exposed:     true,
			Metadata: ToolMetadata{
				Exposure:   ToolExposureDefault,
				SearchHint: "inspect local files",
				SearchTags: []string{"filesystem"},
			},
		},
	}}

	tool := NewToolSearchTool(registry, nil)
	results := runToolSearch(t, tool, ToolSearchParams{Query: "code-search"})

	require.Len(t, results, 1)
	require.Equal(t, "sourcegraph", results[0].Name)
}

func TestToolSearchKeywordRequiredTerms(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{
			Name:        "sourcegraph",
			Description: "search public repositories",
			Metadata: ToolMetadata{
				Exposure:   ToolExposureDeferred,
				SearchHint: "search public repositories",
				SearchTags: []string{"code-search", "network"},
			},
		},
		{
			Name:        "view",
			Description: "read local files",
			Exposed:     true,
			Metadata: ToolMetadata{
				Exposure:   ToolExposureDefault,
				SearchHint: "inspect local files",
				SearchTags: []string{"filesystem"},
			},
		},
	}}

	tool := NewToolSearchTool(registry, nil)
	results := runToolSearch(t, tool, ToolSearchParams{Query: "+network search"})

	require.Len(t, results, 1)
	require.Equal(t, "sourcegraph", results[0].Name)
}

func TestToolSearchNoMatchIncludesPendingMCPServers(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{Name: "view", Description: "read local files", Exposed: true, Metadata: ToolMetadata{Exposure: ToolExposureDefault}},
		{Name: "sourcegraph", Description: "search public repositories", Metadata: ToolMetadata{Exposure: ToolExposureDeferred}},
	}}

	tool := NewToolSearchTool(registry, nil, func() []string {
		return []string{"github", "slack"}
	})
	response := runToolSearchResponse(t, tool, ToolSearchParams{Query: "jira"})

	require.Empty(t, response.Results)
	require.Empty(t, response.Matches)
	require.Equal(t, []string{"github", "slack"}, response.PendingMCPServers)
	require.Equal(t, 1, response.TotalDeferred)
}

func TestToolSearchMCPNameAliasExactMatch(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{
			Name:        "mcp_github_issue_list",
			Description: "list github issues",
			Metadata:    ToolMetadata{Exposure: ToolExposureDeferred},
		},
	}}

	var activated []string
	tool := NewToolSearchTool(registry, func(_ context.Context, toolNames []string) []string {
		activated = append(activated, toolNames...)
		return toolNames
	})

	response := runToolSearchResponse(t, tool, ToolSearchParams{Query: "mcp__github__issue__list"})

	require.Equal(t, []string{"mcp_github_issue_list"}, response.Matches)
	require.Equal(t, []string{"mcp_github_issue_list"}, activated)
	require.Len(t, response.Results, 1)
	require.True(t, response.Results[0].Activated)
}

func TestToolSearchMCPKeywordRanking(t *testing.T) {
	t.Parallel()

	registry := toolSearchRegistryStub{entries: []RegistryEntry{
		{
			Name:        "mcp_github_issue_list",
			Description: "list github issues",
			Metadata: ToolMetadata{
				Exposure:   ToolExposureDeferred,
				SearchHint: "github issue listing",
				SearchTags: []string{"mcp", "github", "issue", "list"},
			},
		},
		{
			Name:        "sourcegraph",
			Description: "search public repositories",
			Metadata: ToolMetadata{
				Exposure:   ToolExposureDeferred,
				SearchHint: "search code in repositories",
				SearchTags: []string{"code-search"},
			},
		},
	}}

	tool := NewToolSearchTool(registry, nil)
	response := runToolSearchResponse(t, tool, ToolSearchParams{Query: "github issue"})

	require.NotEmpty(t, response.Results)
	require.Equal(t, "mcp_github_issue_list", response.Results[0].Name)
}
