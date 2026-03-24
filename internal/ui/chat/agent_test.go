package chat

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestAgentToolMessageItemRendersSubagentTypeAndDescription(t *testing.T) {
	t.Parallel()

	params, err := json.Marshal(agent.AgentParams{
		Description:  "Implement parser worker",
		Prompt:       "Update the parser package and run targeted tests",
		SubagentType: "general",
	})
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	item := NewAgentToolMessageItem(&theme, message.ToolCall{
		ID:       "tool-1",
		Name:     agent.AgentToolName,
		Input:    string(params),
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "tool-1",
		Content:    "done",
	}, false)

	rendered := item.Render(80)
	require.Contains(t, rendered, "General")
	require.Contains(t, rendered, "Implement parser worker")
	require.Contains(t, rendered, "Update the parser package and run targeted tests")
}

func TestAgentToolMessageItemRendersChildSessionStatus(t *testing.T) {
	t.Parallel()

	params, err := json.Marshal(agent.AgentParams{
		Description:  "Implement parser worker",
		Prompt:       "Update the parser package and run targeted tests",
		SubagentType: "general",
	})
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	item := NewAgentToolMessageItem(&theme, message.ToolCall{
		ID:    "tool-1",
		Name:  agent.AgentToolName,
		Input: string(params),
	}, nil, false)
	item.SetChildSessionStatus("Service temporarily unavailable. Retrying in 3 seconds... (attempt 1/5)", false)

	rendered := ansi.Strip(item.Render(120))
	require.Contains(t, rendered, "Status")
	require.Contains(t, rendered, "Retrying in 3 seconds")
}
