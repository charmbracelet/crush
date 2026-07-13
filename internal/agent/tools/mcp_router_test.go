package tools

import (
	"fmt"
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
	require.Contains(t, result, "select:<exact_name>")
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

	require.Equal(t, "Connected MCP tool inventory:\n- github: 2 tools\n- weather: 1 tools\nSearch by capability before selecting a native tool.", result)
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

func TestFormatMCPToolSearchPaginatesWithoutCatalogCeiling(t *testing.T) {
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

	page := formatMCPToolSearch(available, MCPToolSearchParams{Query: "search", MaxResults: 5, Offset: 5})
	require.Contains(t, page, "total_matches: 24")
	require.Contains(t, page, "returned: 5")
	require.Contains(t, page, "offset: 5")
	require.Contains(t, page, "next_offset: 10")
	require.Contains(t, page, "mcp_catalog_search_05")
	require.NotContains(t, page, "mcp_catalog_search_04")

	all := formatMCPToolSearch(available, MCPToolSearchParams{Query: "search", MaxResults: 24})
	require.Contains(t, all, "returned: 24")
	require.NotContains(t, all, "next_offset:")
}
