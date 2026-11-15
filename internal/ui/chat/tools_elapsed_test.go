package chat

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestBaseToolMessageItemElapsed(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	start := time.Now().Add(-5 * time.Second).UnixMilli()
	tc := message.ToolCall{ID: "toolu_1", Name: "bash", StartedAtMs: start}
	item := newBaseToolMessageItem(&sty, tc, nil, &BashToolRenderContext{}, false)
	require.GreaterOrEqual(t, item.elapsed(), 5*time.Second, "a live tool counts from its recorded start")

	// Arguments finishing is not the tool finishing: the timer keeps running.
	tc.Finished = true
	item.SetToolCall(tc)
	require.True(t, item.isSpinning(), "a call whose arguments are done is still running")
	require.GreaterOrEqual(t, item.elapsed(), 5*time.Second)

	item.SetResult(&message.ToolResult{ToolCallID: "toolu_1", FinishedAtMs: start + 7_000})
	require.Equal(t, 7*time.Second, item.elapsed(), "a reply fixes the duration")

	item.SetResult(&message.ToolResult{ToolCallID: "toolu_1"})
	require.Zero(t, item.elapsed(), "a reply with no recorded end has no duration")
}

func TestBaseToolMessageItemElapsedUnknownStart(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "old", Name: "bash", Finished: true}
	item := newBaseToolMessageItem(&sty, tc, &message.ToolResult{ToolCallID: "old"}, &BashToolRenderContext{}, false)
	require.Zero(t, item.elapsed(), "rows stored before timing was recorded show no duration")

	canceled := newBaseToolMessageItem(&sty, message.ToolCall{ID: "c", StartedAtMs: 1}, nil, &BashToolRenderContext{}, true)
	require.Zero(t, canceled.elapsed(), "a canceled call stops counting until its reply lands")
}

func TestBaseToolMessageItemSpinnerTimer(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "toolu_1", Name: "view", StartedAtMs: time.Now().UnixMilli()}
	item := newBaseToolMessageItem(&sty, tc, nil, &ViewToolRenderContext{}, false)

	// A quick tool with no active turn has nothing to show yet.
	require.Empty(t, item.spinnerTimer(), "no turn active, no timer")

	// With a turn active, a quick tool shows the overall turn time.
	common.StartTurn("session")
	defer common.StopTurn("session")
	require.Eventually(t, func() bool {
		return item.spinnerTimer() == common.Elapsed("session") && item.spinnerTimer() != ""
	}, time.Second, time.Millisecond, "quick tools show the overall turn time")
	require.Less(t, item.elapsed(), toolTimerOwnTimeThreshold,
		"the tool itself is still quick, so its own time must not be shown")

	// Once a tool has been running long enough, its own elapsed time
	// takes over the suffix.
	tc.StartedAtMs = time.Now().Add(-15 * time.Second).UnixMilli()
	item.SetToolCall(tc)
	require.Equal(t, common.FormatDuration(item.elapsed()), item.spinnerTimer(),
		"long-running tools show their own elapsed time")
}
