package chat

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
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
