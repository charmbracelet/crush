package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/mcpname"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// The arrow belongs between the server and the tool. Parsing the old
// single-underscore spelling left the server half empty, so it rendered as a
// leading arrow with the whole name, server included, as the tool.
func TestMCPToolHeaderPutsArrowBetweenServerAndTool(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewMCPToolMessageItem(&sty, message.ToolCall{
		ID:    "c1",
		Name:  "mcp__prickly__javascript_eval",
		Input: `{"text":"1+1"}`,
	}, nil, false)

	out := stripStyle(item.Render(120))
	require.Contains(t, out, "Prickly "+styles.ArrowRightIcon+" Javascript Eval")
	require.NotContains(t, out, styles.ArrowRightIcon+" Prickly Javascript Eval",
		"the server must not be swallowed into the tool name")
}

// Docker MCP calls get their own renderer whether they were saved under the
// legacy single-underscore name or the current one.
func TestDockerMCPToolRendersBothSpellings(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	for _, name := range []string{"mcp_docker_mcp-add", "mcp__docker__mcp-add"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.True(t, IsDockerMCPTool(name))
			item := NewToolMessageItem(&sty, "m1", message.ToolCall{
				ID:    "c1",
				Name:  name,
				Input: `{"name":"github"}`,
			}, nil, false, t.TempDir())

			out := stripStyle(item.Render(120))
			require.Contains(t, out, "Docker MCP "+styles.ArrowRightIcon+" Add")
		})
	}

	require.False(t, IsDockerMCPTool("mcp__dockerx__mcp-add"))
	// "docker." cleans to "docker", so its name is hashed and must not pass
	// for the Docker server.
	require.False(t, IsDockerMCPTool(mcpname.Tool("docker.", "mcp-add")))
	require.False(t, IsDockerMCPTool("bash"))
}
