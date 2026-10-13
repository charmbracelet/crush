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
	tc := message.ToolCall{ID: "toolu_1", Name: "bash", Finished: false}
	item := newBaseToolMessageItem(&sty, tc, nil, &BashToolRenderContext{}, false)

	// Windows clocks tick in ~0.5ms steps, so the first read can land on
	// the same tick as construction; poll briefly instead of assuming
	// sub-millisecond resolution.
	require.Eventually(t, func() bool {
		return item.elapsed() > 0
	}, time.Second, time.Millisecond, "a live tool must report elapsed time")

	item.SetToolCall(message.ToolCall{ID: "toolu_1", Name: "bash", Finished: true})
	require.False(t, item.finishedAt.IsZero(), "finishing must capture the end time")

	// Re-reads right after finishing can still land on the same coarse
	// clock tick; only a later read must not grow.
	frozen := item.elapsed()
	time.Sleep(10 * time.Millisecond)
	require.LessOrEqual(t, item.elapsed(), frozen, "elapsed must stop growing once finished")

	item.markRestored()
	require.Zero(t, item.elapsed(), "restored items have no trustworthy start time")
}

func TestBaseToolMessageItemSpinnerTimer(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "toolu_1", Name: "view", Finished: false}
	item := newBaseToolMessageItem(&sty, tc, nil, &ViewToolRenderContext{}, false)

	// A quick tool with no active turn has nothing to show yet.
	require.Empty(t, item.spinnerTimer(), "no turn active, no timer")

	// With a turn active, a quick tool shows the overall turn time.
	common.StartTurn()
	defer common.StopTurn()
	require.Eventually(t, func() bool {
		return item.spinnerTimer() == common.Elapsed() && item.spinnerTimer() != ""
	}, time.Second, time.Millisecond, "quick tools show the overall turn time")
	require.Less(t, item.elapsed(), toolTimerOwnTimeThreshold,
		"the tool itself is still quick, so its own time must not be shown")

	// Once a tool has been running long enough, its own elapsed time
	// takes over the suffix.
	item.startedAt = time.Now().Add(-15 * time.Second)
	require.Equal(t, common.FormatDuration(item.elapsed()), item.spinnerTimer(),
		"long-running tools show their own elapsed time")
}
