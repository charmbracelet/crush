package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

func testTool() *Tool {
	return &Tool{mcpName: "prickly", tool: &mcp.Tool{Name: "prickly_computer"}}
}

// RunTool drops unusable payloads before this point. This backstop keeps a
// future producer from reopening the hole, since the provider refuses an
// empty media block by failing the whole request.
func TestEmptyMediaNeverReachesTheProvider(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"image", "media"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			resp := testTool().responseFor(
				mcp.ToolResult{Type: kind, Data: []byte{}, MediaType: "image/png"},
				true, "claude",
			)
			require.Equal(t, "text", resp.Type, "an empty payload must not go out as media")
			require.Empty(t, resp.Data)
			require.False(t, resp.IsError, "the call succeeded, so a retry would only repeat it")
			require.Equal(t, mcp.EmptyMediaNote, resp.Content)
		})
	}
}

// The capability check comes first, so the reason given is the real one.
func TestModelWithoutImageSupportIsToldFirst(t *testing.T) {
	t.Parallel()

	resp := testTool().responseFor(
		mcp.ToolResult{Type: "image", Data: []byte{}, MediaType: "image/png"},
		false, "some-text-only-model",
	)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "some-text-only-model")
}
