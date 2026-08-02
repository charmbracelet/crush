package tools

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

const testA2UIPayload = `{"version":"v0.9","updateComponents":{"surfaceId":"s","components":[` +
	`{"component":"Text","id":"t","text":"hi"}]}}`

func TestSplitMCPResourceContents(t *testing.T) {
	t.Parallel()

	t.Run("a2ui text content is diverted to metadata", func(t *testing.T) {
		t.Parallel()
		textParts, meta := splitMCPResourceContents([]*mcp.ResourceContents{
			{URI: "cairn://run/x/a2ui", MIMEType: a2uiMIMEType, Text: testA2UIPayload},
		}, true)
		require.Len(t, meta.A2UISurfaces, 1)
		require.Equal(t, "<a2ui-json>"+testA2UIPayload+"</a2ui-json>", meta.A2UISurfaces[0])
		require.Len(t, textParts, 1)
		require.True(t, strings.HasPrefix(textParts[0], A2UISurfacePlaceholderPrefix))
		require.NotContains(t, textParts[0], "updateComponents")
	})

	t.Run("a2ui blob content is diverted too", func(t *testing.T) {
		t.Parallel()
		// Blob is a legal delivery for any MIME type — a blob-delivered
		// surface must not leak raw JSON into the model-facing content.
		textParts, meta := splitMCPResourceContents([]*mcp.ResourceContents{
			{URI: "cairn://run/x/a2ui", MIMEType: a2uiMIMEType, Blob: []byte(testA2UIPayload)},
		}, true)
		require.Len(t, meta.A2UISurfaces, 1)
		require.Len(t, textParts, 1)
		require.True(t, strings.HasPrefix(textParts[0], A2UISurfacePlaceholderPrefix))
		require.NotContains(t, textParts[0], "updateComponents")
	})

	t.Run("no diversion when no UI renders metadata", func(t *testing.T) {
		t.Parallel()
		// Channel-originated and disable_a2ui turns keep the raw payload so
		// the model can relay or summarize the data.
		textParts, meta := splitMCPResourceContents([]*mcp.ResourceContents{
			{URI: "cairn://run/x/a2ui", MIMEType: a2uiMIMEType, Text: testA2UIPayload},
		}, false)
		require.Empty(t, meta.A2UISurfaces)
		require.Equal(t, []string{testA2UIPayload}, textParts)
	})

	t.Run("plain text passes through", func(t *testing.T) {
		t.Parallel()
		textParts, meta := splitMCPResourceContents([]*mcp.ResourceContents{
			{URI: "cairn://artifact/x", MIMEType: "text/markdown", Text: "# hello"},
		}, true)
		require.Empty(t, meta.A2UISurfaces)
		require.Equal(t, []string{"# hello"}, textParts)
	})

	t.Run("mixed text and a2ui keeps both", func(t *testing.T) {
		t.Parallel()
		textParts, meta := splitMCPResourceContents([]*mcp.ResourceContents{
			{URI: "cairn://artifact/x", MIMEType: "text/markdown", Text: "summary"},
			{URI: "cairn://artifact/x/a2ui", MIMEType: a2uiMIMEType, Text: testA2UIPayload},
		}, true)
		require.Len(t, meta.A2UISurfaces, 1)
		require.Len(t, textParts, 2)
		require.Equal(t, "summary", textParts[0])
		require.True(t, strings.HasPrefix(textParts[1], A2UISurfacePlaceholderPrefix))
	})

	t.Run("nil and empty entries are skipped", func(t *testing.T) {
		t.Parallel()
		textParts, meta := splitMCPResourceContents([]*mcp.ResourceContents{
			nil,
			{URI: "cairn://artifact/empty"},
		}, true)
		require.Empty(t, meta.A2UISurfaces)
		require.Empty(t, textParts)
	})
}
