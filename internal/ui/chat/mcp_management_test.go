package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestMCPManagementToolsUseBuiltinRenderer(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	for _, toolName := range []string{tools.MCPAddToolName, tools.MCPRefreshToolName} {
		toolName := toolName
		t.Run(toolName, func(t *testing.T) {
			t.Parallel()

			item := NewToolMessageItem(&sty, "message", message.ToolCall{
				ID:       "call-" + toolName,
				Name:     toolName,
				Input:    `{}`,
				Finished: true,
			}, &message.ToolResult{Content: "ok"}, false)

			rendered := strings.ToLower(item.RawRender(120))
			require.NotContains(t, rendered, "invalid tool name")
			require.Contains(t, rendered, strings.ReplaceAll(toolName, "_", " "))
		})
	}
}
