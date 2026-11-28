package chat

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// countBumps calls Advance n times and reports how many calls bumped the
// item's version. Like the list, it re-renders the item after each change,
// so a clock tick-over bumps once instead of on every later frame.
func countBumps(t *testing.T, item ToolMessageItem, n int) int {
	t.Helper()
	a := item.(Animatable)
	v := item.(versionedItem)
	bumps := 0
	for range n {
		before := v.Version()
		changed := a.Advance()
		require.Equal(t, changed, v.Version() > before, "Advance must bump exactly when it reports a change")
		if changed {
			bumps++
			item.Render(100)
		}
	}
	return bumps
}

// A running bash call whose input is complete draws no spinner, so frames
// only re-render it when its timer text moves on, not on every frame.
func TestRunningToolWithoutSpinnerBumpsOnlyOnTimerChange(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{
		ID:          "run1",
		Name:        tools.BashToolName,
		Input:       `{"command":"go test ./..."}`,
		Finished:    true,
		StartedAtMs: time.Now().Add(-3 * time.Second).UnixMilli(),
	}
	item := NewBashToolMessageItem(&sty, tc, nil, false, "").(*BashToolMessageItem)
	require.True(t, item.Spinning())

	// Before anything is rendered, a frame still bumps.
	require.Equal(t, 1, countBumps(t, item, 1))

	require.Contains(t, ansi.Strip(item.Render(100)), "Waiting for tool response")
	require.False(t, item.animDrawn, "a running call with complete input draws no spinner")

	// The tool's clock and the turn clock can each tick over at most
	// once while these run, and nothing else changes the output.
	require.LessOrEqual(t, countBumps(t, item, 40), 2)

	// A stale timer makes the next frame bump and the next render draw
	// again.
	item.renderedTimer = "stale"
	require.Equal(t, 1, countBumps(t, item, 1))
	item.Render(100)
	require.Equal(t, item.timerKey(), item.renderedTimer)

	// Live output still shows up straight away.
	require.True(t, item.SetLiveOutput("PASS"))
	require.Contains(t, ansi.Strip(item.Render(100)), "PASS")
}

// While the input streams the spinner is drawn, so every frame bumps.
func TestStreamingToolBumpsEveryFrame(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "stream1", Name: tools.BashToolName, Input: `{"comm`}
	item := NewBashToolMessageItem(&sty, tc, nil, false, "").(*BashToolMessageItem)
	for range 10 {
		item.Render(100)
		require.True(t, item.animDrawn)
		require.Equal(t, 1, countBumps(t, item, 1))
	}
}
