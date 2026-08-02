package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

const a2uiTestPlaceholder = tools.A2UISurfacePlaceholderPrefix +
	"mcp://cairn/run/abc123/a2ui — the user can already see it; do not repeat or echo its JSON payload]"

func genericToolOpts(t *testing.T, result *message.ToolResult) *ToolRenderOpts {
	t.Helper()
	return genericToolOptsFor(t, tools.ReadMCPResourceToolName, result)
}

func genericToolOptsFor(t *testing.T, name string, result *message.ToolResult) *ToolRenderOpts {
	t.Helper()
	return &ToolRenderOpts{
		ToolCall: message.ToolCall{
			ID:       "call-1",
			Name:     name,
			Input:    `{"mcp_name":"cairn","uri":"mcp://cairn/run/abc123/a2ui"}`,
			Finished: true,
		},
		Result: result,
		Status: ToolStatusSuccess,
	}
}

// The A2UI payload lives in result metadata (UI-only), not in the content
// the model sees — the model only gets a placeholder, so it cannot echo the
// JSON back and double-render the surface.
func TestGenericToolRendersA2UIFromMetadata(t *testing.T) {
	t.Parallel()

	meta, err := json.Marshal(tools.ReadMCPResourceResponseMetadata{
		A2UISurfaces: []string{a2uiSurface},
	})
	require.NoError(t, err)

	sty := styles.CharmtonePantera()
	ctx := &GenericToolRenderContext{}
	out := ctx.RenderTool(&sty, 80, genericToolOpts(t, &message.ToolResult{
		Content:  a2uiTestPlaceholder,
		Metadata: string(meta),
	}))
	plain := ansi.Strip(out)

	require.Contains(t, plain, "Hello from A2UI")
	require.NotContains(t, plain, "a2ui-json")
	require.NotContains(t, plain, "updateComponents")
	// The model-facing placeholder is not shown to the user.
	require.NotContains(t, plain, "do not repeat")
}

// A result with no A2UI metadata renders its text content as before.
func TestGenericToolNoMetadataRendersText(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &GenericToolRenderContext{}
	out := ctx.RenderTool(&sty, 80, genericToolOpts(t, &message.ToolResult{
		Content: "plain resource text",
	}))
	plain := ansi.Strip(out)

	require.Contains(t, plain, "plain resource text")
}

// A mixed resource read (text/markdown alongside application/a2ui+json)
// shows both the surface and the real text — only the model-facing
// placeholder lines are stripped.
func TestGenericToolMetadataMixedContentShowsText(t *testing.T) {
	t.Parallel()

	meta, err := json.Marshal(tools.ReadMCPResourceResponseMetadata{
		A2UISurfaces: []string{a2uiSurface},
	})
	require.NoError(t, err)

	sty := styles.CharmtonePantera()
	ctx := &GenericToolRenderContext{}
	out := ctx.RenderTool(&sty, 80, genericToolOpts(t, &message.ToolResult{
		Content:  "the artifact summary text\n" + a2uiTestPlaceholder,
		Metadata: string(meta),
	}))
	plain := ansi.Strip(out)

	require.Contains(t, plain, "Hello from A2UI")
	require.Contains(t, plain, "the artifact summary text")
	require.NotContains(t, plain, "do not repeat")
}

// When every metadata surface fails to render, the user sees an alert — not
// the model-facing placeholder claiming the surface is already visible.
func TestGenericToolMetadataAllFailShowsAlert(t *testing.T) {
	t.Parallel()

	meta, err := json.Marshal(tools.ReadMCPResourceResponseMetadata{
		A2UISurfaces: []string{"<a2ui-json>{malformed</a2ui-json>"},
	})
	require.NoError(t, err)

	sty := styles.CharmtonePantera()
	ctx := &GenericToolRenderContext{}
	out := ctx.RenderTool(&sty, 80, genericToolOpts(t, &message.ToolResult{
		Content:  a2uiTestPlaceholder,
		Metadata: string(meta),
	}))
	plain := ansi.Strip(out)

	require.Contains(t, plain, "couldn't render this resource's UI surface")
	require.NotContains(t, plain, "do not repeat")
}

// A failed surface next to a healthy sibling still surfaces an alert — the
// model was told the user can see every surface.
func TestGenericToolMetadataPartialFailShowsAlert(t *testing.T) {
	t.Parallel()

	meta, err := json.Marshal(tools.ReadMCPResourceResponseMetadata{
		A2UISurfaces: []string{a2uiSurface, "<a2ui-json>{malformed</a2ui-json>"},
	})
	require.NoError(t, err)

	sty := styles.CharmtonePantera()
	ctx := &GenericToolRenderContext{}
	out := ctx.RenderTool(&sty, 80, genericToolOpts(t, &message.ToolResult{
		Content:  a2uiTestPlaceholder,
		Metadata: string(meta),
	}))
	plain := ansi.Strip(out)

	require.Contains(t, plain, "Hello from A2UI")
	require.Contains(t, plain, "couldn't render this resource's UI surface")
}

// The content-scan fallback exists for pre-change persisted read_mcp_resource
// results only — other tools sharing the generic renderer must keep
// payload-shaped text as text.
func TestGenericToolFallbackScopedToReadMCPResource(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &GenericToolRenderContext{}
	out := ctx.RenderTool(&sty, 80, genericToolOptsFor(t, "crush_logs", &message.ToolResult{
		Content: "log line before\n" + a2uiSurface + "\nlog line after",
	}))
	plain := ansi.Strip(out)

	// Rendered as text (the raw payload stays visible), not as a surface.
	require.Contains(t, plain, "log line before")
	require.NotContains(t, plain, "Hello from A2UI")
}

// A fenced A2UI example inside a read_mcp_resource result is documentation,
// not live UI (#6, #96) — the masked scan must not extract it.
func TestGenericToolFallbackIgnoresFencedExample(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &GenericToolRenderContext{}
	out := ctx.RenderTool(&sty, 80, genericToolOpts(t, &message.ToolResult{
		Content: "Example:\n\n```json\n" + a2uiSurface + "\n```\n\nDone.",
	}))
	plain := ansi.Strip(out)

	require.Contains(t, plain, "Example:")
	require.NotContains(t, plain, "Hello from A2UI")
}

// Pre-change persisted results interleave prose with tagged surfaces — the
// fallback render keeps the prose.
func TestGenericToolFallbackPreservesProse(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &GenericToolRenderContext{}
	out := ctx.RenderTool(&sty, 80, genericToolOpts(t, &message.ToolResult{
		Content: "intro prose\n" + a2uiSurface + "\nfooter prose",
	}))
	plain := ansi.Strip(out)

	require.Contains(t, plain, "Hello from A2UI")
	require.Contains(t, plain, "intro prose")
	require.Contains(t, plain, "footer prose")
}

// tallA2UISurface builds a surface whose rendered height exceeds the
// collapsed tool-body budget.
func tallA2UISurface(t *testing.T) string {
	t.Helper()
	var comps []string
	var children []string
	for i := 0; i < 20; i++ {
		id := string(rune('a' + i%26))
		id += string(rune('0' + i/26))
		children = append(children, `"`+id+`"`)
		comps = append(comps, `{"component":"Text","id":"`+id+`","text":"row `+id+`"}`)
	}
	comps = append([]string{
		`{"component":"Card","id":"root","child":"col"}`,
		`{"component":"Column","id":"col","children":[` + strings.Join(children, ",") + `]}`,
	}, comps...)
	return `<a2ui-json>{"version":"v0.9","updateComponents":{"surfaceId":"s","components":[` +
		strings.Join(comps, ",") + `]}}</a2ui-json>`
}

// A tall surface honors the same collapsed-height budget as every other tool
// body, and expanding restores the full surface.
func TestGenericToolA2UICollapse(t *testing.T) {
	t.Parallel()

	meta, err := json.Marshal(tools.ReadMCPResourceResponseMetadata{
		A2UISurfaces: []string{tallA2UISurface(t)},
	})
	require.NoError(t, err)

	sty := styles.CharmtonePantera()
	ctx := &GenericToolRenderContext{}

	collapsed := genericToolOpts(t, &message.ToolResult{
		Content:  a2uiTestPlaceholder,
		Metadata: string(meta),
	})
	out := ansi.Strip(ctx.RenderTool(&sty, 80, collapsed))
	require.Contains(t, out, "lines hidden")

	expanded := genericToolOpts(t, &message.ToolResult{
		Content:  a2uiTestPlaceholder,
		Metadata: string(meta),
	})
	expanded.ExpandedContent = true
	outExpanded := ansi.Strip(ctx.RenderTool(&sty, 80, expanded))
	require.NotContains(t, outExpanded, "lines hidden")
	require.Contains(t, outExpanded, "row a0")
}
