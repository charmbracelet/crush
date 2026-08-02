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

func genericToolOpts(t *testing.T, result *message.ToolResult) *ToolRenderOpts {
	t.Helper()
	return &ToolRenderOpts{
		ToolCall: message.ToolCall{
			ID:       "call-1",
			Name:     tools.ReadMCPResourceToolName,
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
		Content:  "[A2UI surface rendered in the chat UI]",
		Metadata: string(meta),
	}))
	plain := ansi.Strip(out)

	require.Contains(t, plain, "Hello from A2UI")
	require.NotContains(t, plain, "a2ui-json")
	require.NotContains(t, plain, "updateComponents")
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
