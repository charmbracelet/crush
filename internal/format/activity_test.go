package format

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestActivityPresenterPlainOutputAndReconciliation(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{
		Output: &output,
		Width:  100,
	})
	thinkingStarted := notify.RunActivity{MessageID: "assistant", Type: notify.RunActivityThinkingStarted}
	thinkingFinished := notify.RunActivity{
		MessageID:        "assistant",
		Type:             notify.RunActivityThinkingFinished,
		ThinkingDuration: 2 * time.Second,
	}
	toolFinished := notify.RunActivity{
		MessageID: "assistant",
		Type:      notify.RunActivityToolFinished,
		ToolCall: message.ToolCall{
			ID:       "call",
			Name:     "view",
			Input:    `{"file_path":"/tmp/run.go","limit":180,"offset":1}`,
			Finished: true,
		},
		ToolResult: message.ToolResult{ToolCallID: "call", Name: "view"},
	}

	presenter.Handle(thinkingStarted)
	presenter.Handle(thinkingStarted)
	presenter.Handle(thinkingFinished)
	presenter.Handle(thinkingFinished)
	toolStarted := toolFinished
	toolStarted.Type = notify.RunActivityToolStarted
	toolStarted.ToolResult = message.ToolResult{}
	presenter.Handle(toolStarted)
	presenter.Handle(toolFinished)
	presenter.Reconcile([]notify.RunActivity{thinkingFinished, toolFinished})
	presenter.Close()

	got := output.String()
	require.Equal(t, 1, strings.Count(got, "✦ Thinking…\n"))
	require.Equal(t, 1, strings.Count(got, "✦ Thought for 2s\n"))
	require.Contains(t, got, "View")
	require.Contains(t, got, "run.go")
	require.Contains(t, got, "limit=180, offset=1")
	require.Equal(t, 2, strings.Count(got, "View"))
	require.Less(t, strings.Index(got, "●"), strings.Index(got, "✓"))
	require.NotContains(t, got, "\x1b")
}

func TestActivityPresenterToolErrorsAndTruncation(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{Output: &output, Width: 50})
	presenter.Handle(notify.RunActivity{
		MessageID: "assistant",
		Type:      notify.RunActivityToolFinished,
		ToolCall: message.ToolCall{
			ID:       "call",
			Name:     "grep",
			Input:    `{"pattern":"a very long pattern that must be truncated rather than written on another line","path":"/tmp"}`,
			Finished: true,
		},
		ToolResult: message.ToolResult{ToolCallID: "call", IsError: true},
	})
	presenter.Close()

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	require.Len(t, lines, 1)
	require.LessOrEqual(t, lipgloss.Width(lines[0]), 48)
	require.Contains(t, lines[0], "…")
}

func TestActivityPresenterClosedIsSilent(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	presenter := NewActivityPresenter(context.Background(), func() {}, ActivityPresenterOptions{Output: &output})
	presenter.Close()
	presenter.Handle(notify.RunActivity{
		MessageID:        "assistant",
		Type:             notify.RunActivityThinkingFinished,
		ThinkingDuration: time.Second,
	})
	require.Empty(t, output.String())
}

func TestActivityPresenterBashStripsWorkspacePrefix(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{
		Output:     &output,
		WorkingDir: "/workspace",
		Width:      100,
	})
	activity := notify.RunActivity{
		MessageID: "assistant",
		Type:      notify.RunActivityToolStarted,
		ToolCall: message.ToolCall{
			ID:       "call",
			Name:     "bash",
			Input:    `{"command":"cd /workspace && go test ./..."}`,
			Finished: true,
		},
	}
	presenter.Handle(activity)
	activity.Type = notify.RunActivityToolFinished
	activity.ToolResult = message.ToolResult{ToolCallID: "call", Name: "bash"}
	presenter.Handle(activity)
	presenter.Close()

	got := output.String()
	require.NotContains(t, got, "cd /workspace && ")
	require.Equal(t, 2, strings.Count(got, "Bash go test ./..."))
}
