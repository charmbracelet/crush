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
		out, failed := item.renderToolA2UISurfaces(80)
		plain := ansi.Strip(out)
		require.Contains(t, plain, "Recipe Card")
		require.NotContains(t, plain, "updateComponents")
		require.Zero(t, failed)
	})

	t.Run("a failed sibling payload still alerts next to a live surface", func(t *testing.T) {
		t.Parallel()
		// One payload renders, one is malformed: the live path must show
		// the rendered surface AND the alert — the model was told the
		// user can see both.
		item := newA2UIToolItem(t,
			[]string{a2uiToolSurface, "<a2ui-json>{malformed</a2ui-json>"},
			[]string{"cairn", "cairn"}, tools.ReadMCPResourceToolName)
		item.syncToolA2UISurfaces()
		require.True(t, item.hasToolA2UISurfaces())
		_, failed := item.renderToolA2UISurfaces(80)
		require.Equal(t, 1, failed)

		out := ansi.Strip(item.RawRender(100))
		require.Contains(t, out, "Recipe Card")
		require.Contains(t, out, "couldn't render this resource's UI surface")
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

// TestToolA2UIClearCacheDropsSurfacesForRestyle pins the theme-change path for
// tool-result surfaces. buildA2UISurfaces bakes the active theme's
// render.Styles into each a2tea model, and clearCache is only ever called from
// the theme-change path (applyTheme -> refreshStyles ->
// InvalidateRenderCaches -> ClearItemCaches), so a surface kept across it goes
// on drawing the previous palette beside newly-themed chat. The assistant item
// has had this drop since the surfaces landed; the tool item did not.
func TestToolA2UIClearCacheDropsSurfacesForRestyle(t *testing.T) {
	t.Parallel()

	item := newA2UIToolItem(t, []string{a2uiToolSurface}, []string{"recipe"}, "mcp_recipe_card")
	_ = item.RawRender(100)
	require.True(t, item.hasToolA2UISurfaces(), "surfaces must build on first render")
	require.True(t, item.surfaceScanned)

	item.clearCache()

	require.False(t, item.surfaceScanned, "the scan guard must reset so the next render rebuilds")
	require.Zero(t, item.surfaceSrcHash, "a kept hash would short-circuit the rebuild")
	require.False(t, item.hasToolA2UISurfaces(), "stale themed models must be dropped")

	// The surfaces come back on the next render, now built from current styles.
	_ = item.RawRender(100)
	require.True(t, item.hasToolA2UISurfaces(), "surfaces must rebuild after a restyle")
}

// TestToolRenderDoesNotCacheLiveSurfaceFrame pins that Render never writes a
// frame containing a live surface into the prefixed-render cache. The surfaces
// are built inside RawRender, so the hasToolA2UISurfaces() check that guards
// the cache is still false when Render first consults it — caching there froze
// the surface's opening frame, and getCachedPrefixedRender keys on
// (width, prefix) alone, so once the server deleted the surface the dead frame
// was served for the rest of the item's life.
func TestToolRenderDoesNotCacheLiveSurfaceFrame(t *testing.T) {
	t.Parallel()

	// The tool name must not itself contain the surface's text, or the
	// rendered header would satisfy the replay assertion below.
	item := newA2UIToolItem(t, []string{a2uiToolSurface}, []string{"recipe"}, "mcp_recipe_fetch")

	_ = item.Render(100)
	require.True(t, item.hasToolA2UISurfaces(), "the render must have built a live surface")
	require.Empty(t, item.prefixedRendered,
		"a frame holding a live surface must not be cached")

	// Drop the surface the way a server-sent deleteSurface does. With the
	// frame wrongly cached, the next Render would replay it and keep drawing
	// the surface that no longer exists.
	item.dropToolA2UISurfaces()
	item.result.Metadata = ""
	out := item.Render(100)
	require.NotContains(t, ansi.Strip(out), "Recipe Card",
		"a deleted surface must not be replayed from the cache")
}
