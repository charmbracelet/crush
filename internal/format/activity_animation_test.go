//go:build !windows

package format

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/tone"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestActivityThinkingIsStatic(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	presenter := &ActivityPresenter{sty: &sty}
	presenter.activities = []activityLine{{kind: activityFrameThinking, startedAt: time.Now()}}
	for _, frame := range []uint64{0, 1, 2, 3} {
		presenter.animationFrame = frame
		require.Equal(t, "✦ Thinking…", ansi.Strip(presenter.activityText(presenter.activities[0])))
	}
}

func TestActivityHistoryHasBlankLineBeforeGenerating(t *testing.T) {
	var output spinnerBuffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{Output: &output, TTY: true})
	defer presenter.Close()
	presenter.completeActivity(activityFrameThinking, "assistant", "✦ Thought for 2s")
	output.Reset()
	presenter.completeActivity(activityFrameTool, "call", "✓ View")
	got := strings.ReplaceAll(ansi.Strip(output.String()), "\r", "")
	require.Equal(t, "✓ View\n\nGenerating…", got)
}

func TestActivityHistoryHasBlankLineBeforeOutput(t *testing.T) {
	var output spinnerBuffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{Output: &output, TTY: true})
	defer presenter.Close()
	presenter.completeActivity(activityFrameThinking, "assistant", "✦ Thought for 2s")
	output.Reset()
	presenter.TextStarted()
	got := strings.ReplaceAll(ansi.Strip(output.String()), "\r", "")
	require.Equal(t, "✦ Thought for 2s\n\n", got)
}

func TestActivityToolUsesBlinkingDot(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	presenter := &ActivityPresenter{sty: &sty}
	presenter.activities = []activityLine{{
		kind: activityFrameTool,
		text: ToolSummaryWithStatus(&sty, message.ToolCall{Name: "view", Input: `{"file_path":"main.go"}`}, ToolStatusRunning, 100, ""),
	}}
	presenter.activities[0].startedAt = time.Now()
	visible := presenter.activityText(presenter.activities[0])
	require.Contains(t, visible, lipgloss.NewStyle().Foreground(tone.Julep).Render("●"))
	presenter.animationFrame = 1
	darker := presenter.activityText(presenter.activities[0])
	require.Contains(t, darker, lipgloss.NewStyle().Foreground(tone.Guac).Render("●"))
	require.NotEqual(t, visible, darker)
}

func TestActivityTransitionsPreserveCompletedLines(t *testing.T) {
	var output spinnerBuffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{
		Output: &output,
		TTY:    true,
		Width:  100,
	})
	defer presenter.Close()
	thinkingStarted := notify.RunActivity{MessageID: "assistant", Type: notify.RunActivityThinkingStarted}
	thinkingFinished := thinkingStarted
	thinkingFinished.Type = notify.RunActivityThinkingFinished
	thinkingFinished.ThinkingDuration = 2 * time.Second
	presenter.Handle(thinkingStarted)
	presenter.Handle(thinkingFinished)

	toolStarted := notify.RunActivity{
		MessageID: "assistant",
		Type:      notify.RunActivityToolStarted,
		ToolCall: message.ToolCall{
			ID:       "call",
			Name:     "view",
			Input:    `{"file_path":"main.go"}`,
			Finished: true,
		},
	}
	output.Reset()
	presenter.Handle(toolStarted)
	require.Contains(t, output.String(), ansi.CursorUp(1)+"\r"+ansi.EraseDisplay(0), "new activity must replace only the spacer, preserving completed predecessors")

	secondTool := toolStarted
	secondTool.ToolCall.ID = "call-2"
	presenter.Handle(secondTool)
	require.Len(t, presenter.activities, 3)

	toolFinished := toolStarted
	toolFinished.Type = notify.RunActivityToolFinished
	toolFinished.ToolResult = message.ToolResult{ToolCallID: "call", Name: "view"}
	output.Reset()
	presenter.Handle(toolFinished)
	require.Contains(t, output.String(), ansi.CursorUp(3), "completion must update the running tool line in the activity region, including the spacer row")
	require.True(t, presenter.activities[1].finished)
	require.False(t, presenter.activities[2].finished)
	require.Equal(t, "call-2", presenter.activities[2].id)

	presenter.Handle(notify.RunActivity{MessageID: "second", Type: notify.RunActivityThinkingStarted})
	presenter.Handle(notify.RunActivity{MessageID: "second", Type: notify.RunActivityThinkingFinished})
	output.Reset()
	presenter.TextStarted()
	time.Sleep(100 * time.Millisecond)
	require.NotContains(t, output.String(), "Generating", "text streaming hides the bottom spinner")
}

func TestActivityAppendClearOnlyTheProgressSpacer(t *testing.T) {
	var output spinnerBuffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{Output: &output, TTY: true})
	defer presenter.Close()
	presenter.completeActivity(activityFrameThinking, "assistant", "✦ Thought for 2s")
	presenter.completeActivity(activityFrameTool, "call-one", "✓ View")
	output.Reset()
	index, added := presenter.upsert(activityFrameTool, "call-two", "✓ Grep", true)
	presenter.displayActivity(index, added)

	got := output.String()
	require.True(t, strings.HasPrefix(got, ansi.CursorUp(1)+"\r"+ansi.EraseDisplay(0)), "append must clear the old spacer, not history")
	require.Equal(t, 1, strings.Count(got, "\n\n"), "there is exactly one spacer, after the latest activity")
}

func TestActivityDoesNotRedrawCompletedOrIdleLines(t *testing.T) {
	var output spinnerBuffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{Output: &output, TTY: true})
	defer presenter.Close()
	output.Reset()
	presenter.animateChangedLines()
	require.Empty(t, output.String(), "idle progress text must remain static")

	presenter.Handle(notify.RunActivity{MessageID: "assistant", Type: notify.RunActivityThinkingStarted})
	presenter.Handle(notify.RunActivity{
		MessageID:        "assistant",
		Type:             notify.RunActivityThinkingFinished,
		ThinkingDuration: time.Second,
	})
	output.Reset()
	presenter.animateChangedLines()
	require.Empty(t, output.String(), "completed activity must not be repainted")
}

func TestActivityAddsOutputAfterTextWithoutErasingIt(t *testing.T) {
	var output spinnerBuffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{
		Output: &output,
		TTY:    true,
		Width:  100,
	})
	defer presenter.Close()
	started := notify.RunActivity{
		MessageID: "assistant",
		Type:      notify.RunActivityToolStarted,
		ToolCall: message.ToolCall{
			ID:       "call",
			Name:     "view",
			Input:    `{"file_path":"main.go"}`,
			Finished: true,
		},
	}
	presenter.Handle(started)
	presenter.TextStarted()
	before := output.String()
	_, _ = fmt.Fprint(&output, "complete assistant response\n")

	finished := started
	finished.Type = notify.RunActivityToolFinished
	finished.ToolResult = message.ToolResult{ToolCallID: "call", Name: "view"}
	presenter.Handle(finished)
	presenter.Handle(notify.RunActivity{MessageID: "second", Type: notify.RunActivityThinkingStarted})
	require.Contains(t, output.String(), "complete assistant response\n")
	require.NotContains(t, output.String()[len(before):], ansi.CursorUp(1), "later activity must not redraw over streamed output")
}

func TestStopProgressPreservesRunningActivities(t *testing.T) {
	var output spinnerBuffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{Output: &output, TTY: true})
	defer presenter.Close()
	started := notify.RunActivity{
		MessageID: "assistant",
		Type:      notify.RunActivityToolStarted,
		ToolCall: message.ToolCall{
			ID:       "call",
			Name:     "view",
			Input:    `{"file_path":"main.go"}`,
			Finished: true,
		},
	}
	presenter.Handle(started)
	presenter.StopProgress()
	require.Len(t, presenter.activities, 1)
	require.False(t, presenter.activities[0].finished)

	finished := started
	finished.Type = notify.RunActivityToolFinished
	finished.ToolResult = message.ToolResult{ToolCallID: "call", Name: "view"}
	presenter.Reconcile([]notify.RunActivity{finished})
	require.Len(t, presenter.activities, 1)
	require.True(t, presenter.activities[0].finished)
}

func TestThinkingAlwaysBecomesThought(t *testing.T) {
	var output spinnerBuffer
	presenter := NewActivityPresenter(t.Context(), func() {}, ActivityPresenterOptions{Output: &output, TTY: true})
	defer presenter.Close()
	presenter.Handle(notify.RunActivity{MessageID: "assistant", Type: notify.RunActivityThinkingStarted})
	presenter.Handle(notify.RunActivity{MessageID: "assistant", Type: notify.RunActivityThinkingFinished})
	require.Len(t, presenter.activities, 1)
	require.True(t, presenter.activities[0].finished)
	require.Contains(t, ansi.Strip(presenter.activities[0].text), "Thought for 0s")
}
