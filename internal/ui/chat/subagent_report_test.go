package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/require"
)

var (
	_ HighlightableMessageItem = (*SubagentReportItem)(nil)
	_ FocusableMessageItem     = (*SubagentReportItem)(nil)
	_ Expandable               = (*SubagentReportItem)(nil)
)

func reportMessage(report message.SubagentReport) *message.Message {
	return &message.Message{
		ID:    "report-msg",
		Role:  message.User,
		Parts: []message.ContentPart{report},
	}
}

func TestExtractMessageItemsRoutesSubagentReports(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := reportMessage(message.SubagentReport{
		Name:   "code-reviewer-1",
		Status: subagents.StatusCompleted,
		Output: "No blocking issues found.",
	})

	items := ExtractMessageItems(&sty, msg, nil, "")
	require.Len(t, items, 1)
	require.IsType(t, &SubagentReportItem{}, items[0])
	require.Equal(t, msg.ID, items[0].ID())

	rendered := ansi.Strip(items[0].Render(80))
	require.Contains(t, rendered, "code-reviewer-1")
	require.Contains(t, rendered, "No blocking issues found.")
}

func TestExtractMessageItemsKeepsPlainUserMessages(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:    "user-msg",
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
	}

	items := ExtractMessageItems(&sty, msg, nil, "")
	require.Len(t, items, 1)
	require.IsType(t, &UserMessageItem{}, items[0])
}

func TestSubagentReportItemHeaderShowsNameAndStatus(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	for _, status := range []string{
		subagents.StatusCompleted,
		subagents.StatusFailed,
		subagents.StatusCancelled,
	} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()

			item := NewSubagentReportItem(&sty, reportMessage(message.SubagentReport{
				Name:   "tester-2",
				Status: status,
				Output: "body text",
			}))

			lines := strings.Split(ansi.Strip(item.Render(80)), "\n")
			require.Contains(t, lines[0], "tester-2")
			require.Contains(t, lines[0], status)
		})
	}
}

func TestSubagentReportItemGolden(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tests := []struct {
		name   string
		report message.SubagentReport
	}{
		{"completed", message.SubagentReport{
			Name:   "code-reviewer-1",
			Status: subagents.StatusCompleted,
			Output: "Reviewed **3 files**.\n\n- No blocking issues\n- One nit in `main.go`",
		}},
		{"failed", message.SubagentReport{
			Name:   "tester-1",
			Status: subagents.StatusFailed,
			Output: "The test suite could not start: missing fixtures.",
		}},
		{"cancelled", message.SubagentReport{
			Name:   "planner-1",
			Status: subagents.StatusCancelled,
			Output: "Stopped: Crush exited",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item := NewSubagentReportItem(&sty, reportMessage(tt.report))
			golden.RequireEqual(t, []byte(item.Render(80)))
		})
	}
}

func TestSubagentReportItemCollapsesLongOutput(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	const total = 60
	var out strings.Builder
	for i := range total {
		fmt.Fprintf(&out, "- finding-%02d\n", i)
	}
	item := NewSubagentReportItem(&sty, reportMessage(message.SubagentReport{
		Name:   "code-reviewer-1",
		Status: subagents.StatusCompleted,
		Output: out.String(),
	}))

	collapsed := ansi.Strip(item.Render(80))
	require.Contains(t, collapsed, "finding-00")
	require.NotContains(t, collapsed, fmt.Sprintf("finding-%02d", total-1))

	expandable, ok := item.(Expandable)
	require.True(t, ok)
	require.True(t, expandable.ToggleExpanded())
	expanded := ansi.Strip(item.Render(80))
	require.Contains(t, expanded, fmt.Sprintf("finding-%02d", total-1))

	require.False(t, expandable.ToggleExpanded())
	require.NotContains(t, ansi.Strip(item.Render(80)), fmt.Sprintf("finding-%02d", total-1))
}

func TestSubagentReportItemFitsNarrowWidth(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewSubagentReportItem(&sty, reportMessage(message.SubagentReport{
		Name:   "code-reviewer-1",
		Status: subagents.StatusFailed,
		Output: strings.Repeat("a very long line of report output ", 8),
	}))

	const width = 40
	for _, line := range strings.Split(item.Render(width), "\n") {
		require.LessOrEqual(t, ansi.StringWidth(line), width, "line overflows: %q", ansi.Strip(line))
	}
}
