package mcp

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestToolResultFromCallKeepsIsError(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		result  *mcp.CallToolResult
		kind    string
		content string
	}{
		{
			name: "text error",
			result: &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: "upstream refused this call"}},
				StructuredContent: map[string]any{"error": "upstream_boom"},
				IsError:           true,
			},
			kind:    "text",
			content: "upstream refused this call",
		},
		{
			name:    "error without content",
			result:  &mcp.CallToolResult{IsError: true},
			kind:    "text",
			content: "",
		},
		{
			name: "image error",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "screenshot failed"},
					&mcp.ImageContent{Data: []byte("SGVsbG8="), MIMEType: "image/png"},
				},
				IsError: true,
			},
			kind:    "image",
			content: "screenshot failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := toolResultFromCall(tt.result)
			require.True(t, got.IsError)
			require.Equal(t, tt.kind, got.Type)
			require.Equal(t, tt.content, got.Content)
		})
	}
}

func TestToolResultFromCallSuccessIsNotError(t *testing.T) {
	t.Parallel()

	got := toolResultFromCall(&mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
	})
	require.False(t, got.IsError)
	require.Equal(t, "ok", got.Content)
}
