package tools

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

func TestSplitMCPToolResult(t *testing.T) {
	t.Parallel()

	surface := mcp.A2UISurface{
		Payload:          testA2UIPayload,
		URI:              "a2ui://recipe-card",
		AssistantVisible: true,
	}

	t.Run("surfaces divert to metadata with provenance", func(t *testing.T) {
		t.Parallel()
		content, meta := splitMCPToolResult(mcp.ToolResult{
			Type:     "text",
			Content:  "Your recipe card",
			Surfaces: []mcp.A2UISurface{surface},
		}, "recipe", true)

		require.Len(t, meta.A2UISurfaces, 1)
		require.Equal(t, "<a2ui-json>"+testA2UIPayload+"</a2ui-json>", meta.A2UISurfaces[0])
		require.Equal(t, []string{"recipe"}, meta.MCPSurfaceProvenance)
		require.Contains(t, content, "Your recipe card")
		require.True(t, strings.Contains(content, A2UISurfacePlaceholderPrefix))
		require.NotContains(t, content, "updateComponents")
	})

	t.Run("no diversion folds the payload back for the model", func(t *testing.T) {
		t.Parallel()
		// Channel-originated turns keep the payload so the model can relay it.
		content, meta := splitMCPToolResult(mcp.ToolResult{
			Type:     "text",
			Content:  "fallback text",
			Surfaces: []mcp.A2UISurface{surface},
		}, "recipe", false)

		require.Empty(t, meta.A2UISurfaces)
		require.Contains(t, content, "fallback text")
		require.Contains(t, content, testA2UIPayload)
	})

	t.Run("user-only payload stays hidden when not diverting", func(t *testing.T) {
		t.Parallel()
		userOnly := surface
		userOnly.AssistantVisible = false
		content, meta := splitMCPToolResult(mcp.ToolResult{
			Type:     "text",
			Content:  "fallback text",
			Surfaces: []mcp.A2UISurface{userOnly},
		}, "recipe", false)

		require.Empty(t, meta.A2UISurfaces)
		require.Equal(t, "fallback text", content)
	})

	t.Run("surface without fallback text still gets a placeholder", func(t *testing.T) {
		t.Parallel()
		content, meta := splitMCPToolResult(mcp.ToolResult{
			Type:     "text",
			Content:  "",
			Surfaces: []mcp.A2UISurface{surface},
		}, "recipe", true)

		require.Len(t, meta.A2UISurfaces, 1)
		require.True(t, strings.HasPrefix(content, A2UISurfacePlaceholderPrefix))
	})
}
