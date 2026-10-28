package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// The display name is a pure function of the label, so the dispatch, the
// report, and any follow-up all show the same person without sharing state.
func TestSubagentDisplayName(t *testing.T) {
	t.Parallel()
	name := SubagentDisplayName("date-impl")
	require.True(t, strings.HasSuffix(name, "-date-impl"), name)
	require.Equal(t, name, SubagentDisplayName("date-impl"))
	require.Empty(t, SubagentDisplayName(""))
}

func renderPlain(t *testing.T, item MessageItem) []string {
	t.Helper()
	return strings.Split(ansi.Strip(item.RawRender(100)), "\n")
}

// A report's header names the subagent and its model, and its body starts
// in the same column as the header's text rather than indented past it.
func TestSubagentReportAlignsWithHeader(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	item := NewSubAgentReportItem(&sty, "date-impl", "hyper/glm-5", "All done.", false)
	lines := renderPlain(t, item)

	name := SubagentDisplayName("date-impl")
	require.Contains(t, lines[0], "Subagent Report "+name+" (model=hyper/glm-5)")
	headerText := ansi.StringWidth(lines[0][:strings.Index(lines[0], "Subagent Report")])
	body := lines[2]
	require.Equal(t, headerText, len(body)-len(strings.TrimLeft(body, " ")), "body: %q", body)
}

// A follow-up names its subagent and shows the message as prose, not the
// call's JSON or the delivery receipt.
func TestAgentSendShowsTheMessage(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
		ID:       "call-1",
		Name:     agent.AgentSendToolName,
		Input:    `{"label":"date-impl","message":"Please fix the lint."}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "call-1",
		Content:    `Delivered to "date-impl", which had finished.`,
	}, false, "/tmp")
	out := strings.Join(renderPlain(t, item), "\n")

	require.Contains(t, out, "Message Subagent → "+SubagentDisplayName("date-impl"))
	require.Contains(t, out, "Please fix the lint.")
	require.NotContains(t, out, `"label"`)
	require.NotContains(t, out, "Delivered")
}

// The dispatch header shows the model the subagent actually runs on, which
// the dispatch result records even without an override.
func TestDispatchHeaderShowsRunModel(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
		ID:       "call-1",
		Name:     agent.AgentDispatchToolName,
		Input:    `{"prompt":"look","label":"audit"}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "call-1",
		Content:    `Started subagent "audit".`,
		Metadata:   `{"label":"audit","run_model":"hyper/glm-5"}`,
	}, false, "/tmp")
	lines := renderPlain(t, item)
	require.Contains(t, lines[0], "Create Subagent "+SubagentDisplayName("audit")+" (model=hyper/glm-5)")
}

// A long model name on a narrow pane is what hid the override's model: the
// shared header drops key=value pairs once they leave under 30 columns.
func TestSubagentHeaderKeepsLongModelOnNarrowPane(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	h := ansi.Strip(subagentHeader(&sty, ToolStatusSuccess, "Create Subagent", "smoke-override", "anthropic/claude-haiku-4-5-20251001", 72, nil))
	require.Contains(t, h, "(model=")
	require.LessOrEqual(t, ansi.StringWidth(h), 72)
}

// agent_status draws one aligned row per subagent from its metadata.
func TestAgentStatusRendersRows(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
		ID: "call-1", Name: agent.AgentStatusToolName, Input: `{}`, Finished: true,
	}, &message.ToolResult{
		ToolCallID: "call-1",
		Content:    "- a: finished",
		Metadata:   `{"agents":[{"label":"a","state":"finished","timing":"5m ago","access":"read","tool_calls":2},{"label":"long-label","state":"running","timing":"for 1m","access":"write","tool_calls":1}]}`,
	}, false, "/tmp")
	lines := renderPlain(t, item)

	require.NotContains(t, lines[0], "→", "a status of every subagent names no target")
	a, long := lines[2], lines[3]
	require.Contains(t, a, "✓ "+SubagentDisplayName("a"))
	require.Contains(t, a, "finished 5m ago · 2 tool calls")
	require.Contains(t, long, "running for 1m · 1 tool calls · can write")
	require.Equal(t, strings.Index(a, "finished"), strings.Index(long, "running"), "states line up")
}

// A stopped subagent was stopped on purpose, so it reads as canceled, not failed.
func TestAgentStateIconStoppedIsNotAnError(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	require.Equal(t, toolIcon(&sty, ToolStatusCanceled), agentStateIcon(&sty, "stopped"))
	require.Equal(t, toolIcon(&sty, ToolStatusError), agentStateIcon(&sty, "failed"))
}

// Markdown output ends with its last line of text, not a painted blank line.
func TestSubagentReportHasNoBlankTail(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	for _, out := range []string{"Done.\n\nAll good.", "Contents:\n\n```\na\nb\n```"} {
		lines := renderPlain(t, NewSubAgentReportItem(&sty, "x", "", out, false))
		require.NotEmpty(t, strings.TrimSpace(lines[len(lines)-1]), "%q", out)
	}
}
