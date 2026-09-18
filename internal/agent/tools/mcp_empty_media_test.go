package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

func testTool() *Tool {
	return &Tool{mcpName: "prickly", tool: &mcp.Tool{Name: "prickly_computer"}}
}

// TestEmptyMediaNeverReachesTheProvider is the last gate before a payload
// leaves for the provider. An empty one is refused for the whole request
// rather than just the attachment, and the rejected block then sits in the
// conversation failing every turn after it, so the session cannot be used
// again. RunTool drops empty media before this point; this keeps a future
// producer from reopening the same hole.
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
			require.True(t, resp.IsError)
			require.Contains(t, resp.Content, "empty media")
		})
	}
}

// TestRealMediaStillPasses guards against closing the empty case by breaking
// the ordinary one.
func TestRealMediaStillPasses(t *testing.T) {
	t.Parallel()

	png := []byte{0x89, 'P', 'N', 'G'}
	resp := testTool().responseFor(
		mcp.ToolResult{Type: "image", Data: png, MediaType: "image/png", Content: "a screenshot"},
		true, "claude",
	)

	require.Equal(t, "image", resp.Type)
	require.False(t, resp.IsError)
	require.Equal(t, png, resp.Data)
	require.Equal(t, "image/png", resp.MediaType)
	require.Equal(t, "a screenshot", resp.Content)
}

// TestModelWithoutImageSupportIsToldFirst keeps the capability check ahead of
// the emptiness one, so the reason given is the real one.
func TestModelWithoutImageSupportIsToldFirst(t *testing.T) {
	t.Parallel()

	resp := testTool().responseFor(
		mcp.ToolResult{Type: "image", Data: []byte{}, MediaType: "image/png"},
		false, "some-text-only-model",
	)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "some-text-only-model")
}

// TestTextResultIsUntouched covers the ordinary path through the switch.
func TestTextResultIsUntouched(t *testing.T) {
	t.Parallel()

	resp := testTool().responseFor(mcp.ToolResult{Type: "text", Content: "done"}, true, "claude")
	require.False(t, resp.IsError)
	require.Equal(t, "done", resp.Content)
}
