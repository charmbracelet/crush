package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

// TestMCPToolInfoDescribesEveryParameter covers MCP servers that publish a
// bare inputSchema. Without the fallback those properties reach the provider
// undescribed and the whole request is rejected.
func TestMCPToolInfoDescribesEveryParameter(t *testing.T) {
	t.Parallel()

	tool := &Tool{
		mcpName: "demo",
		tool: &mcp.Tool{
			Name:        "create_issue",
			Description: "Create an issue",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"title":  map[string]any{"type": "string"},
					"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"body":   map[string]any{"type": "string", "description": "Issue body"},
				},
				"required": []any{"title"},
			},
		},
	}

	info := tool.Info()

	require.Equal(t, []string{"title"}, info.Required)

	title, ok := info.Parameters["title"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "title", title["description"])

	body, ok := info.Parameters["body"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Issue body", body["description"])

	// Existing schema keywords must not be dropped when a description is added.
	labels, ok := info.Parameters["labels"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "labels", labels["description"])
	require.Equal(t, "array", labels["type"])
	items, ok := labels["items"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "string", items["type"])
}
