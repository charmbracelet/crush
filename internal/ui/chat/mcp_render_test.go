package chat

import (
	"testing"

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
