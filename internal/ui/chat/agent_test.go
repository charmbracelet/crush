package chat

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/toolruntime"
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

func TestAgentToolMessageItemCollapsesNestedToolsByDefault(t *testing.T) {
	t.Parallel()

	params, err := json.Marshal(agent.AgentParams{
		Description:  "Long review",
		Prompt:       "Inspect recent commits",
		SubagentType: "explore",
	})
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	item := NewAgentToolMessageItem(&theme, message.ToolCall{
		ID:       "agent-tool",
		Name:     agent.AgentToolName,
		Input:    string(params),
		Finished: true,
	}, nil, false)

	for i := 1; i <= 12; i++ {
		item.AddNestedTool(newNestedBashTool(t, &theme, fmt.Sprintf("nested-%02d", i)))
	}

	collapsed := ansi.Strip(item.Render(140))
	require.Contains(t, collapsed, "Expand (2 more)")
	require.Contains(t, collapsed, "nested-10")
	require.NotContains(t, collapsed, "nested-11")
	require.NotContains(t, collapsed, "nested-12")

	item.ToggleExpanded()
	expanded := ansi.Strip(item.Render(140))
	require.Contains(t, expanded, "Collapse")
	require.Contains(t, expanded, "nested-11")
	require.Contains(t, expanded, "nested-12")
}

func TestAgenticFetchToolMessageItemCollapsesNestedToolsByDefault(t *testing.T) {
	t.Parallel()

	params, err := json.Marshal(agenticFetchParams{
		Prompt: "Collect package docs",
	})
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	item := NewAgenticFetchToolMessageItem(&theme, message.ToolCall{
		ID:       "agentic-fetch-tool",
		Name:     tools.AgenticFetchToolName,
		Input:    string(params),
		Finished: true,
	}, nil, false)

	for i := 1; i <= 11; i++ {
		item.AddNestedTool(newNestedBashTool(t, &theme, fmt.Sprintf("fetch-nested-%02d", i)))
	}

	collapsed := ansi.Strip(item.Render(140))
	require.Contains(t, collapsed, "Expand (1 more)")
	require.NotContains(t, collapsed, "fetch-nested-11")

	item.ToggleExpanded()
	expanded := ansi.Strip(item.Render(140))
	require.Contains(t, expanded, "Collapse")
	require.Contains(t, expanded, "fetch-nested-11")
}

func newNestedBashTool(t *testing.T, sty *styles.Styles, cmd string) ToolMessageItem {
	t.Helper()

	input, err := json.Marshal(tools.BashParams{Command: cmd, Description: "nested"})
	require.NoError(t, err)

	return NewBashToolMessageItem(sty, message.ToolCall{
		ID:       "nested-" + cmd,
		Name:     tools.BashToolName,
		Input:    string(input),
		Finished: true,
	}, nil, false)
}

func TestBashToolMessageItemRuntimeSnapshotRendersSanitizedText(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(tools.BashParams{Command: "echo test", Description: "runtime"})
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	item := NewBashToolMessageItem(&theme, message.ToolCall{
		ID:    "tool-runtime",
		Name:  tools.BashToolName,
		Input: string(input),
	}, nil, false)
	item.SetRuntimeState(&toolruntime.State{
		ToolCallID:   "tool-runtime",
		ToolName:     tools.BashToolName,
		Status:       toolruntime.StatusRunning,
		SnapshotText: "3\nwarn",
	})

	rendered := ansi.Strip(item.Render(100))
	require.Contains(t, rendered, "3")
	require.Contains(t, rendered, "warn")
	require.NotContains(t, rendered, "Waiting for tool response...")
	require.NotContains(t, rendered, "\x1b")
}
