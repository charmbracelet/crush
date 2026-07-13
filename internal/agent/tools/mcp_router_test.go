package tools

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestFormatMCPToolSearchReturnsOnlyRelevantSchemas(t *testing.T) {
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

	require.Contains(t, result, "server: weather")
	require.Contains(t, result, "tool: get_forecast")
	require.Contains(t, result, `"location_name"`)
	require.NotContains(t, result, "get_file_contents")
}

func TestFormatMCPToolSearchEmptyQueryReturnsCompactInventory(t *testing.T) {
	t.Parallel()

	available := []*Tool{
		{mcpName: "github", tool: &mcp.Tool{Name: "get_me"}},
		{mcpName: "github", tool: &mcp.Tool{Name: "get_file_contents"}},
		{mcpName: "weather", tool: &mcp.Tool{Name: "get_forecast"}},
	}

	result := formatMCPToolSearch(available, MCPToolSearchParams{})

	require.Equal(t, "Connected MCP tool inventory:\n- github: 2 tools\n- weather: 1 tools\nSearch by capability before calling a tool.", result)
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
