package chat

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

const a2uiToolSurface = `<a2ui-json>{"version":"v0.9","updateComponents":{"surfaceId":"card","components":[` +
	`{"component":"Card","id":"root","child":"col"},` +
	`{"component":"Column","id":"col","children":["title"]},` +
	`{"component":"Text","id":"title","variant":"h2","text":"Recipe Card"}]}}` + `</a2ui-json>`

// newA2UIToolItem builds a base tool item whose result metadata carries the
// given surfaces and provenance — the shape read_mcp_resource and mcp_* tool
// results produce.
func newA2UIToolItem(t *testing.T, surfaces, provenance []string, toolName string) *baseToolMessageItem {
	t.Helper()
	meta, err := json.Marshal(tools.ReadMCPResourceResponseMetadata{
		A2UISurfaces:         surfaces,
		MCPSurfaceProvenance: provenance,
	})
	require.NoError(t, err)

	sty := styles.CharmtonePantera()
	return newBaseToolMessageItem(&sty, message.ToolCall{
		ID:       "call-1",
		Name:     toolName,
		Input:    `{}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "call-1",
		Content:    a2uiTestPlaceholder,
		Metadata:   string(meta),
	}, &GenericToolRenderContext{}, false)
}

func TestToolA2UISurfaces(t *testing.T) {
	t.Parallel()

	t.Run("metadata surfaces build live models", func(t *testing.T) {
		t.Parallel()
		item := newA2UIToolItem(t, []string{a2uiToolSurface}, []string{"cairn"}, tools.ReadMCPResourceToolName)
		item.syncToolA2UISurfaces()

		require.True(t, item.hasToolA2UISurfaces())
		require.Len(t, item.a2ui.surfaces, 1)
		require.Equal(t, "card", item.a2ui.surfaceIDs[0])
	})

	t.Run("provenance is registered per surface", func(t *testing.T) {
		t.Parallel()
		item := newA2UIToolItem(t, []string{a2uiToolSurface}, []string{"cairn"}, tools.ReadMCPResourceToolName)
		item.syncToolA2UISurfaces()

		server, ok := A2UISurfaceProvenance("card")
		require.True(t, ok)
		require.Equal(t, "cairn", server)
	})

	t.Run("missing provenance means unknown origin", func(t *testing.T) {
		t.Parallel()
		item := newA2UIToolItem(t, []string{a2uiToolSurface}, nil, tools.ReadMCPResourceToolName)
		item.syncToolA2UISurfaces()
		require.True(t, item.hasToolA2UISurfaces())
	})

	t.Run("same metadata reuses live models", func(t *testing.T) {
		t.Parallel()
		item := newA2UIToolItem(t, []string{a2uiToolSurface}, []string{"cairn"}, tools.ReadMCPResourceToolName)
		item.syncToolA2UISurfaces()
		first := item.a2ui.surfaces[0]

		// A re-render with the same result must reuse the model so focus
		// and edited field values survive.
		item.syncToolA2UISurfaces()
		require.Same(t, first, item.a2ui.surfaces[0])
	})

	t.Run("malformed payload yields no live surface", func(t *testing.T) {
		t.Parallel()
		item := newA2UIToolItem(t, []string{"<a2ui-json>{malformed</a2ui-json>"}, []string{"cairn"}, tools.ReadMCPResourceToolName)
		item.syncToolA2UISurfaces()
		require.False(t, item.hasToolA2UISurfaces())
	})

	t.Run("rendering a live surface draws its content", func(t *testing.T) {
		t.Parallel()
		item := newA2UIToolItem(t, []string{a2uiToolSurface}, []string{"cairn"}, tools.ReadMCPResourceToolName)
		item.syncToolA2UISurfaces()
		out := item.renderToolA2UISurfaces(80)
		plain := ansi.Strip(out)
		require.Contains(t, plain, "Recipe Card")
		require.NotContains(t, plain, "updateComponents")
	})

	t.Run("RawRender renders the surface instead of the placeholder", func(t *testing.T) {
		t.Parallel()
		item := newA2UIToolItem(t, []string{a2uiToolSurface}, []string{"cairn"}, tools.ReadMCPResourceToolName)
		out := ansi.Strip(item.RawRender(100))
		require.Contains(t, out, "Recipe Card")
		require.NotContains(t, out, "do not repeat")
	})
}

// The MCP tool renderer (mcp_* tools) renders metadata surfaces too, not
// just read_mcp_resource.
func TestMCPToolRendersA2UIFromMetadata(t *testing.T) {
	t.Parallel()

	meta, err := json.Marshal(tools.ReadMCPResourceResponseMetadata{
		A2UISurfaces: []string{a2uiToolSurface},
	})
	require.NoError(t, err)

	sty := styles.CharmtonePantera()
	ctx := &MCPToolRenderContext{}
	out := ctx.RenderTool(&sty, 80, genericToolOptsFor(t, "mcp_recipe_get_recipe_a2ui", &message.ToolResult{
		Content:  a2uiTestPlaceholder,
		Metadata: string(meta),
	}))
	plain := ansi.Strip(out)

	require.Contains(t, plain, "Recipe Card")
	require.NotContains(t, plain, "do not repeat")
}
