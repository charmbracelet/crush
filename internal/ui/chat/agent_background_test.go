package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

const backgroundStartedContent = `Started in the background as "reviewer-1". Its report arrives when it finishes.`

// backgroundAgentItem builds an agent tool item whose call returned
// immediately because the subagent was started in the background.
func backgroundAgentItem(t *testing.T) *AgentToolMessageItem {
	t.Helper()

	sty := styles.CharmtonePantera()
	call := message.ToolCall{
		ID:       "call-1",
		Name:     "agent",
		Input:    `{"prompt":"review the diff","subagent_type":"code-reviewer"}`,
		Finished: true,
	}
	result := &message.ToolResult{
		ToolCallID: "call-1",
		Name:       "agent",
		Content:    backgroundStartedContent,
		Metadata:   `{"name":"reviewer-1","background":true}`,
	}
	return NewAgentToolMessageItem(&sty, call, result, false)
}

func TestAgentToolItemRendersBackgroundLabelInsteadOfStartedText(t *testing.T) {
	t.Parallel()

	item := backgroundAgentItem(t)

	rendered := ansi.Strip(item.Render(100))
	lines := strings.Split(rendered, "\n")
	require.Contains(t, lines[0], "Background: reviewer-1")
	require.Contains(t, rendered, "review the diff")
	require.NotContains(t, rendered, "Started in the background")
}

func TestAgentToolItemBackgroundKeepsNestedTools(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := backgroundAgentItem(t)
	childCall := message.ToolCall{ID: "child-1", Name: "glob", Input: `{"pattern":"*.go"}`, Finished: true}
	item.AddNestedTool(NewToolMessageItem(&sty, "msg", childCall, &message.ToolResult{ToolCallID: "child-1", Content: "x"}, false, ""))

	require.Contains(t, ansi.Strip(item.Render(100)), "*.go")
}

func TestAgentToolItemSpinsWhileBackgroundSubagentRuns(t *testing.T) {
	t.Parallel()

	item := backgroundAgentItem(t)

	item.SetBackgroundRunning(true)
	require.True(t, item.Spinning())
	require.False(t, item.Finished())
	require.True(t, item.Advance(), "a running background subagent must animate")

	item.SetBackgroundRunning(false)
	require.False(t, item.Spinning())
	require.True(t, item.Finished())
	require.False(t, item.Advance())
}

func TestAgentSendAndStopToolsRenderReadably(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tests := []struct {
		name   string
		tool   string
		input  string
		result string
	}{
		{"send", "agent_send", `{"name":"reviewer-1","message":"also check the tests"}`, `Message delivered to "reviewer-1".`},
		{"stop", "agent_stop", `{"name":"reviewer-1"}`, `Stopped "reviewer-1".`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			call := message.ToolCall{ID: "call-1", Name: tt.tool, Input: tt.input, Finished: true}
			result := &message.ToolResult{ToolCallID: "call-1", Name: tt.tool, Content: tt.result}
			item := NewToolMessageItem(&sty, "msg", call, result, false, "")

			rendered := ansi.Strip(item.Render(100))
			lines := strings.Split(rendered, "\n")
			require.Contains(t, lines[0], "reviewer-1")
			require.NotContains(t, lines[0], `{"`, "the header must not show raw JSON input")
			require.Contains(t, rendered, tt.result)
		})
	}
}
