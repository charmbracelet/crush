package tools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestFormatMCPToolSearchReturnsCompactRelevantMatches(t *testing.T) {
	t.Parallel()

	weather := &Tool{
		mcpName: "weather",
		tool: &mcp.Tool{
			Name:        "get_forecast",
			Description: "Get a weather forecast for a saved location.",
			InputSchema: map[string]any{
				"properties": map[string]any{"location_name": map[string]any{"type": "string"}},
				"required":   []any{"location_name"},
			},
		},
	}
	github := &Tool{
		mcpName: "github",
		tool: &mcp.Tool{
			Name:        "get_file_contents",
			Description: "Read a file from a GitHub repository.",
			InputSchema: map[string]any{"properties": map[string]any{"path": map[string]any{"type": "string"}}},
		},
	}

	result := formatMCPToolSearch([]*Tool{github, weather}, MCPToolSearchParams{Query: "Bangkok forecast"})

	require.Contains(t, result, "name: mcp_weather_get_forecast")
	require.Contains(t, result, "server: weather")
	require.NotContains(t, result, `"location_name"`)
	require.NotContains(t, result, "get_file_contents")
	require.Contains(t, result, "available on the next model step")
	require.Contains(t, result, "loaded: 1")
	require.NotContains(t, result, "select:<exact_name>")
}

func TestFormatMCPToolSearchRejectsDataIdentifierAsCapability(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "create_branch", Description: "Create a branch in a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "search_repositories", Description: "Find GitHub repositories by metadata."}},
	}

	result := formatMCPToolSearch(available, MCPToolSearchParams{Query: "crush-re", Server: "github"})

	require.Contains(t, result, "No usable MCP tools matched")
}

func TestFormatMCPToolSearchExactSelectionReturnsSchema(t *testing.T) {
	t.Parallel()

	weather := &Tool{
		mcpName: "weather",
		tool: &mcp.Tool{
			Name:        "get_forecast",
			Description: "Get a weather forecast for a saved location.",
			InputSchema: map[string]any{
				"properties": map[string]any{"location_name": map[string]any{"type": "string"}},
				"required":   []any{"location_name"},
			},
		},
	}

	result := formatMCPToolSearch([]*Tool{weather}, MCPToolSearchParams{Query: "select:mcp_weather_get_forecast"})

	require.Contains(t, result, "available on the next model step")
	require.Contains(t, result, "name: mcp_weather_get_forecast")
	require.Contains(t, result, `"location_name"`)
	require.Contains(t, result, "selected: 1")
}

func TestDeferredToolSelectionNames(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		[]string{"mcp_github_get_me", "mcp_weather_forecast"},
		DeferredToolSelectionNames(" SELECT:mcp_github_get_me, mcp_weather_forecast,mcp_github_get_me "),
	)
	require.Nil(t, DeferredToolSelectionNames("github tools"))
}

func TestFormatMCPToolSearchEmptyQueryReturnsCompactInventory(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "get_me"}},
		{mcpName: "github", tool: &mcp.Tool{Name: "get_file_contents"}},
		{mcpName: "weather", tool: &mcp.Tool{Name: "get_forecast"}},
	}

	result := formatMCPToolSearch(available, MCPToolSearchParams{})

	require.Contains(t, result, "github:\n- mcp_github_get_file_contents\n- mcp_github_get_me")
	require.Contains(t, result, "weather:\n- mcp_weather_get_forecast")
	require.Contains(t, result, "full descriptions and input schemas load on demand")
	require.NotContains(t, result, `"properties"`)
}

func TestFormatMCPToolSearchServerWithoutQueryRequestsCapability(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "get_me"}},
		{mcpName: "github", tool: &mcp.Tool{Name: "get_file_contents"}},
		{mcpName: "weather", tool: &mcp.Tool{Name: "get_forecast"}},
	}

	result := formatMCPToolSearch(available, MCPToolSearchParams{Server: "github"})

	require.Contains(t, result, "github:\n- mcp_github_get_file_contents\n- mcp_github_get_me")
	require.NotContains(t, result, "mcp_weather_get_forecast")
}

func TestAllowedMCPToolsPreservesAgentRestrictions(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "get_me"}},
		{mcpName: "github", tool: &mcp.Tool{Name: "get_file_contents"}},
		{mcpName: "weather", tool: &mcp.Tool{Name: "get_forecast"}},
	}

	filtered := allowedMCPTools(available, map[string][]string{"github": {"get_me"}})

	require.Len(t, filtered, 1)
	require.Equal(t, "github", filtered[0].MCP())
	require.Equal(t, "get_me", filtered[0].MCPToolName())
}

func TestFormatMCPToolSearchDefaultsToFiveAndAllowsDeliberateExpansion(t *testing.T) {
	t.Parallel()

	available := make([]*Tool, 0, 24)
	for i := range 24 {
		available = append(available, &Tool{
			mcpName: "catalog",
			tool: &mcp.Tool{
				Name:        fmt.Sprintf("search_%02d", i),
				Description: "Search the complete catalog.",
			},
		})
	}

	page := formatMCPToolSearch(available, MCPToolSearchParams{Query: "search"})
	require.Contains(t, page, "total_matches: 24")
	require.Contains(t, page, "loaded: 5")
	require.Contains(t, page, "mcp_catalog_search_00")
	require.NotContains(t, page, "mcp_catalog_search_05")

	expanded := formatMCPToolSearch(available, MCPToolSearchParams{Query: "search", MaxResults: 24})
	require.Contains(t, expanded, "loaded: 24")
	require.Contains(t, expanded, "mcp_catalog_search_23")
}

func TestCompactMCPToolCatalogListsEveryExactNameWithoutSchemas(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "get_me"}},
		{mcpName: "github", tool: &mcp.Tool{Name: "search_repositories"}},
		{mcpName: "weather", tool: &mcp.Tool{Name: "forecast"}},
	}

	catalog := compactMCPToolCatalog(available)
	require.Contains(t, catalog, "github:\n- mcp_github_get_me\n- mcp_github_search_repositories")
	require.Contains(t, catalog, "weather:\n- mcp_weather_forecast")
	require.NotContains(t, catalog, `"properties"`)
}

func TestFormatMCPToolSearchUsesServerNameAsScopeNotBroadMatch(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "create_repository", Description: "Create a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "create_branch", Description: "Create a branch in a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "list_branches", Description: "List branches in a GitHub repository."}},
		{mcpName: "weather", tool: &mcp.Tool{Name: "get_forecast", Description: "Get a weather forecast."}},
	}

	branches := formatMCPToolSearch(available, MCPToolSearchParams{Query: "github branches"})
	require.Contains(t, branches, "mcp_github_create_branch")
	require.Contains(t, branches, "mcp_github_list_branches")
	require.NotContains(t, branches, "mcp_github_create_repository")
	require.NotContains(t, branches, "mcp_weather_get_forecast")

	allGitHub := formatMCPToolSearch(available, MCPToolSearchParams{Query: "github"})
	require.Contains(t, allGitHub, "loaded: 3")
	require.Contains(t, allGitHub, "mcp_github_create_repository")
}

func TestFormatMCPToolSearchNormalizesNaturalPluralQueriesAcrossServers(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "create_repository", Description: "Create a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "search_repositories", Description: "Find GitHub repositories by name or metadata."}},
		{mcpName: "memory", tool: &mcp.Tool{Name: "search_nodes", Description: "Search a knowledge graph for remembered entities."}},
		{mcpName: "playwright", tool: &mcp.Tool{Name: "browser_console_messages", Description: "Read browser console messages."}},
		{mcpName: "weather", tool: &mcp.Tool{Name: "get_forecast", Description: "Get a weather forecast for a location."}},
	}

	repositories := formatMCPToolSearch(available, MCPToolSearchParams{Query: "find repository by name", Server: "github"})
	require.Contains(t, repositories, "mcp_github_search_repositories")

	memory := formatMCPToolSearch(available, MCPToolSearchParams{Query: "search remembered knowledge graph", Server: "memory"})
	require.Contains(t, memory, "mcp_memory_search_nodes")

	browser := formatMCPToolSearch(available, MCPToolSearchParams{Query: "inspect browser console messages", Server: "playwright"})
	require.Contains(t, browser, "mcp_playwright_browser_console_messages")

	weather := formatMCPToolSearch(available, MCPToolSearchParams{Query: "weather forecast Bangkok", Server: "weather"})
	require.Contains(t, weather, "mcp_weather_get_forecast")
}

func TestFormatMCPToolSearchOriginalRepositoryQueryKeepsSearchCapabilityVisible(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "create_repository", Description: "Create a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "fork_repository", Description: "Fork a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "get_file_contents", Description: "Read a file from a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "list_repository_collaborators", Description: "List collaborators for a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "search_repositories", Description: "Find GitHub repositories by name or metadata."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "update_pull_request", Description: "Update a pull request."}},
	}

	result := formatMCPToolSearch(available, MCPToolSearchParams{
		Query:  "repository reitaard/crush-re.configured",
		Server: "github",
	})

	require.Contains(t, result, "mcp_github_search_repositories")
	require.Contains(t, result, "loaded: 5")
}

func TestFormatMCPToolSearchOldBranchQueriesExposeListBranches(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "create_branch", Description: "Create a branch in a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "get_commit", Description: "Get commit details from a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "get_file_contents", Description: "Get file or directory contents from a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "get_label", Description: "Get a label from a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "get_latest_release", Description: "Get the latest release from a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "list_branches", Description: "List branches in a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "list_commits", Description: "List commits on a branch in a GitHub repository."}},
		{mcpName: "github", tool: &mcp.Tool{Name: "search_repositories", Description: "Find GitHub repositories by metadata."}},
	}

	broad := formatMCPToolSearch(available, MCPToolSearchParams{
		Query:  "get repository info branch",
		Server: "github",
	})
	require.Contains(t, broad, "mcp_github_list_branches")

	precise := formatMCPToolSearch(available, MCPToolSearchParams{
		Query:  "list branches",
		Server: "github",
	})
	listIndex := strings.Index(precise, "mcp_github_list_branches")
	createIndex := strings.Index(precise, "mcp_github_create_branch")
	require.NotEqual(t, -1, listIndex)
	require.True(t, createIndex == -1 || listIndex < createIndex)
}

func TestFormatMCPToolSearchExactSelectionIgnoresFuzzyLimit(t *testing.T) {
	t.Parallel()

	var available []*Tool
	var names []string
	for i := range 8 {
		name := fmt.Sprintf("tool_%02d", i)
		available = append(available, &Tool{mcpName: "server", tool: &mcp.Tool{Name: name}})
		names = append(names, "mcp_server_"+name)
	}

	result := formatMCPToolSearch(available, MCPToolSearchParams{
		Query:      "select:" + strings.Join(names, ","),
		MaxResults: 1,
	})

	require.Contains(t, result, "selected: 8")
}

func TestMCPToolSearchInstructionsDescribeAutomaticDeferredWorkflow(t *testing.T) {
	t.Parallel()

	description := strings.Join(strings.Fields(mcpToolSearchDescription), " ")
	require.Contains(t, description, "Every available deferred tool appears by exact name")
	require.Contains(t, description, "select:mcp_github_get_me")
	require.Contains(t, description, "deferred again for the next user turn")
	require.Contains(t, description, "load the `mcp-setup` skill")
}
