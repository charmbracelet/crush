package chat

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/planmode"
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

func TestAgentToolMessageItemRendersTaskListForTaskGraph(t *testing.T) {
	t.Parallel()

	params, err := json.Marshal(agent.AgentParams{
		Tasks: []agent.AgentTaskParams{
			{ID: "t1", Description: "Search references", Prompt: "Find usages", SubagentType: "explore"},
			{ID: "t2", Description: "Apply patch", Prompt: "Implement fix", SubagentType: "general"},
			{ID: "t3", Description: "Run tests", Prompt: "Run targeted tests", SubagentType: "general"},
			{ID: "t4", Description: "Summarize", Prompt: "Write summary", SubagentType: "general"},
		},
	})
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	item := NewAgentToolMessageItem(&theme, message.ToolCall{
		ID:       "tool-taskgraph",
		Name:     agent.AgentToolName,
		Input:    string(params),
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "tool-taskgraph",
		Content:    "done",
	}, false)

	rendered := ansi.Strip(item.Render(120))
	require.Contains(t, rendered, "Tasks")
	require.Contains(t, rendered, "Tasks")
	require.Contains(t, rendered, "done 0 · running 4 · pending 0")
	require.Contains(t, rendered, "[Explore] Search references")
	require.Contains(t, rendered, "[General] Apply patch")
	require.Contains(t, rendered, "[General] Run tests")
	require.Contains(t, rendered, "[General] Summarize")
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

func TestAgentToolMessageItemRendersTaskDependenciesAndStatuses(t *testing.T) {
	t.Parallel()

	params, err := json.Marshal(agent.AgentParams{
		Tasks: []agent.AgentTaskParams{
			{ID: "t1", Description: "Index code", Prompt: "Build map", SubagentType: "explore"},
			{ID: "t2", Description: "Apply fix", Prompt: "Patch", SubagentType: "general", DependsOn: []string{"t1"}},
		},
	})
	require.NoError(t, err)

	resultContent := "All good\n- t1: completed\n- t2: failed\n\nTask outputs:\n- t1 ..."
	theme := styles.DefaultStyles()
	item := NewAgentToolMessageItem(&theme, message.ToolCall{
		ID:       "tool-graph",
		Name:     agent.AgentToolName,
		Input:    string(params),
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "tool-graph",
		Content:    resultContent,
	}, false)

	rendered := ansi.Strip(item.Render(140))
	require.Contains(t, rendered, "done 1 · running 0 · pending 0 · failed 1")
	require.Contains(t, rendered, "[Explore] Index code")
	require.Contains(t, rendered, "[General] Apply fix (after: t1)")
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
		Name:     agenttools.AgenticFetchToolName,
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

	input, err := json.Marshal(agenttools.BashParams{Command: cmd, Description: "nested"})
	require.NoError(t, err)

	return NewBashToolMessageItem(sty, message.ToolCall{
		ID:       "nested-" + cmd,
		Name:     agenttools.BashToolName,
		Input:    string(input),
		Finished: true,
	}, nil, false)
}

func TestBashToolMessageItemRuntimeSnapshotRendersSanitizedText(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(agenttools.BashParams{Command: "echo test", Description: "runtime"})
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	item := NewBashToolMessageItem(&theme, message.ToolCall{
		ID:    "tool-runtime",
		Name:  agenttools.BashToolName,
		Input: string(input),
	}, nil, false)
	item.SetRuntimeState(&toolruntime.State{
		ToolCallID:   "tool-runtime",
		ToolName:     agenttools.BashToolName,
		Status:       toolruntime.StatusRunning,
		SnapshotText: "3\nwarn",
	})

	rendered := ansi.Strip(item.Render(100))
	require.Contains(t, rendered, "3")
	require.Contains(t, rendered, "warn")
	require.NotContains(t, rendered, "Waiting for tool response...")
	require.NotContains(t, rendered, "\x1b")
}

func TestBashToolMessageItemFinalRuntimeSnapshotRendersBeforeToolResultArrives(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(agenttools.BashParams{Command: "git show --stat HEAD", Description: "runtime final"})
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	item := NewBashToolMessageItem(&theme, message.ToolCall{
		ID:       "tool-runtime-final",
		Name:     agenttools.BashToolName,
		Input:    string(input),
		Finished: true,
	}, nil, false)
	item.SetRuntimeState(&toolruntime.State{
		ToolCallID:   "tool-runtime-final",
		ToolName:     agenttools.BashToolName,
		Status:       toolruntime.StatusCompleted,
		SnapshotText: "commit abc123\n file.go | 2 +-",
	})

	rendered := ansi.Strip(item.Render(100))
	require.Contains(t, rendered, "commit abc123")
	require.Contains(t, rendered, "file.go | 2 +-")
	require.NotContains(t, rendered, "Waiting for tool response...")
	require.NotContains(t, rendered, "no output")
	if bashItem, ok := item.(*BashToolMessageItem); ok {
		require.Equal(t, ToolStatusSuccess, bashItem.computeStatus())
	}
}

func TestAssistantMessageOnlyRendersProposedPlanWithPlanExitToolCall(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	content := planmode.WrapProposedPlan("- Step 1")

	withoutPlanExit := message.Message{
		ID:   "assistant-without-plan-exit",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: content},
		},
	}
	item := NewAssistantMessageItem(&theme, &withoutPlanExit)
	rendered := ansi.Strip(item.Render(120))
	require.NotContains(t, rendered, "Proposed Plan")
	require.Contains(t, rendered, "<proposed_plan>")

	withPlanExit := message.Message{
		ID:   "assistant-with-plan-exit",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: content},
			message.ToolCall{ID: "tool-1", Name: agenttools.PlanExitToolName, Finished: true},
		},
	}
	item = NewAssistantMessageItem(&theme, &withPlanExit)
	rendered = ansi.Strip(item.Render(120))
	require.Contains(t, rendered, "Proposed Plan")
	require.NotContains(t, rendered, "<proposed_plan>")
	require.Contains(t, rendered, "Step 1")
}
