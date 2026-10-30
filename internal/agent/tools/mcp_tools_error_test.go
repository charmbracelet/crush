package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

func TestMCPToolErrorResponse(t *testing.T) {
	t.Parallel()

	t.Run("keeps the server's error text", func(t *testing.T) {
		t.Parallel()

		resp := mcpToolErrorResponse(mcp.ToolResult{Type: "text", Content: "upstream refused this call", IsError: true})
		require.True(t, resp.IsError)
		require.Equal(t, "text", resp.Type)
		require.Equal(t, "upstream refused this call", resp.Content)
	})

	t.Run("drops media and reports text", func(t *testing.T) {
		t.Parallel()

		resp := mcpToolErrorResponse(mcp.ToolResult{Type: "image", Content: "screenshot failed", Data: []byte("x"), MediaType: "image/png", IsError: true})
		require.True(t, resp.IsError)
		require.Equal(t, "text", resp.Type)
		require.Empty(t, resp.Data)
		require.Equal(t, "screenshot failed", resp.Content)
	})

	t.Run("fills in empty error text", func(t *testing.T) {
		t.Parallel()

		resp := mcpToolErrorResponse(mcp.ToolResult{Type: "text", IsError: true})
		require.True(t, resp.IsError)
		require.NotEmpty(t, resp.Content)
	})
}
