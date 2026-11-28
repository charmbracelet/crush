package chat

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
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

func TestRunningToolShowsTookInHeader(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{
		ID:          "toolu_1",
		Name:        "view",
		Input:       `{"file_path":"/tmp/a.go"}`,
		Finished:    true,
		StartedAtMs: time.Now().Add(-time.Second).UnixMilli(),
	}
	item := newBaseToolMessageItem(&sty, tc, nil, &ViewToolRenderContext{}, false)
	out := ansi.Strip(item.RawRender(120))
	require.NotContains(t, out, "took", "a quick tool shows no duration")
	require.NotContains(t, out, "Waiting for tool response", "the waiting line is gone")

	tc.StartedAtMs = time.Now().Add(-3 * time.Second).UnixMilli()
	item.SetToolCall(tc)
	require.Contains(t, ansi.Strip(item.RawRender(120)), "took=3s", "a long-running tool shows its own duration")

	// A call still queued behind others has not started, so it has no
	// duration to show.
	tc.StartedAtMs = 0
	item.SetToolCall(tc)
	require.NotContains(t, ansi.Strip(item.RawRender(120)), "took")
}

func TestToolHeaderShowsTookWithoutOtherParams(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	out := ansi.Strip(toolHeader(&sty, ToolStatusRunning, "Agent", 120, &ToolRenderOpts{
		Status:  ToolStatusRunning,
		Elapsed: 3 * time.Second,
	}))
	require.Contains(t, out, "Agent (took=3s)", "no placeholder param is needed for a duration-only header")

	header := toolHeader(&sty, ToolStatusRunning, "View", 120, &ToolRenderOpts{Status: ToolStatusRunning}, "/tmp/a.go")
	block, ok := toolStateBlock(&sty, &ToolRenderOpts{Status: ToolStatusRunning}, 120, header)
	require.True(t, ok)
	require.Equal(t, header, block, "a running tool is its header, with no empty body below it")
}
