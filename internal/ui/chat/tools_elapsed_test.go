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

	require.Greater(t, item.elapsed(), time.Duration(0), "a live tool must report elapsed time")

	item.SetToolCall(message.ToolCall{ID: "toolu_1", Name: "bash", Finished: true})
	require.False(t, item.finishedAt.IsZero(), "finishing must capture the end time")

	frozen := item.elapsed()
	time.Sleep(10 * time.Millisecond)
	require.Equal(t, frozen, item.elapsed(), "elapsed must stop growing once finished")

	item.markRestored()
	require.Zero(t, item.elapsed(), "restored items have no trustworthy start time")
}
