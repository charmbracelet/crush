package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// HasNestedTools must agree with the items NewToolMessageItem builds, since
// a session load reads nested transcripts by tool name before any item
// exists.
func TestHasNestedToolsMatchesToolItems(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	names := []string{
		agent.AgentToolName,
		agent.AgentDispatchToolName,
		agent.AgentSendToolName,
		agent.AgentStatusToolName,
		agent.AgentStopToolName,
		tools.AgenticFetchToolName,
		tools.BashToolName,
		tools.FetchToolName,
		tools.WebFetchToolName,
		tools.WebSearchToolName,
		tools.ViewToolName,
		tools.TodosToolName,
		"mcp_server_tool",
		"unknown",
	}
	for _, name := range names {
		item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
			ID:    "call-1",
			Name:  name,
			Input: `{}`,
		}, nil, false, "/tmp")
		_, isContainer := item.(NestedToolContainer)
		require.Equal(t, isContainer, HasNestedTools(name), name)
	}
}
