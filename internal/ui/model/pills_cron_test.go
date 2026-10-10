package model

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/scheduler"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Scheduled-task pills rendering
//
// The pills panel shows scheduled tasks twice: a count pill on the pills
// row, and an expanded list underneath. Both carry glyphs that also appear
// in the chat tool-call body, so these tests pin the glyphs themselves
// rather than just "renders something" — a styled empty string is visually
// indistinguishable from a missing prefix until you count columns.
//
// @joestump-agent 08/23/2026 - Added the recurring/one-shot prefix.
//
// @joestump 08/23/2026 - Added these tests while fixing cronItemPrefix,
// which rendered the recurring prefix as an empty string because
// Tool.CronRecurringIcon has no SetString.

func testStyles(t *testing.T) *styles.Styles {
	t.Helper()
	s := styles.CharmtonePantera()
	return &s
}

func TestCronItemPrefixRendersDistinctGlyphs(t *testing.T) {
	t.Parallel()
	sty := testStyles(t)

	recurring := ansi.Strip(cronItemPrefix(scheduler.Task{Recurring: true}, sty))
	oneShot := ansi.Strip(cronItemPrefix(scheduler.Task{Recurring: false}, sty))

	assert.Equal(t, styles.CronRecurringIcon, recurring)
	assert.Equal(t, styles.CronOneShotIcon, oneShot)
	assert.NotEqual(t, recurring, oneShot, "recurring and one-shot must be distinguishable")
}

// Both prefixes must occupy the same number of cells or the list ragged-edges.
func TestCronItemPrefixWidthsMatch(t *testing.T) {
	t.Parallel()
	sty := testStyles(t)

	recurring := cronItemPrefix(scheduler.Task{Recurring: true}, sty)
	oneShot := cronItemPrefix(scheduler.Task{Recurring: false}, sty)

	require.NotZero(t, ansi.StringWidth(recurring), "recurring prefix rendered as an empty string")
	assert.Equal(t, ansi.StringWidth(oneShot), ansi.StringWidth(recurring))
}

func TestCronListPrefixesEachTaskByRecurrence(t *testing.T) {
	t.Parallel()
	sty := testStyles(t)
	at := time.Now().Add(time.Hour)

	out := ansi.Strip(cronList([]scheduler.Task{
		{ID: "task-a", Prompt: "check the build", NextRunAt: at, Recurring: true},
		{ID: "task-b", Prompt: "remind me", NextRunAt: at, Recurring: false},
	}, sty))

	lines := strings.Split(out, "\n")
	require.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], styles.CronRecurringIcon+" "), "got %q", lines[0])
	assert.True(t, strings.HasPrefix(lines[1], styles.CronOneShotIcon+" "), "got %q", lines[1])
	assert.Contains(t, lines[0], "task-a")
	assert.Contains(t, lines[1], "task-b")
}

// Recurrence is signalled by the prefix; the old inline suffix would print
// the same glyph a second time on the same row.
func TestCronListDoesNotRepeatRecurringGlyph(t *testing.T) {
	t.Parallel()
	sty := testStyles(t)

	out := ansi.Strip(cronList([]scheduler.Task{
		{ID: "task-a", Prompt: "check the build", NextRunAt: time.Now().Add(time.Hour), Recurring: true},
	}, sty))

	assert.Equal(t, 1, strings.Count(out, styles.CronRecurringIcon), "got %q", out)
}

func TestCronListAndPillEmptyWithoutTasks(t *testing.T) {
	t.Parallel()
	sty := testStyles(t)

	assert.Empty(t, cronList(nil, sty))
	assert.Empty(t, cronPill(nil, sty))
}

func TestCronPillUsesStopwatchGlyphAndCount(t *testing.T) {
	t.Parallel()
	sty := testStyles(t)

	out := ansi.Strip(cronPill([]scheduler.Task{
		{ID: "task-a", NextRunAt: time.Now()},
		{ID: "task-b", NextRunAt: time.Now()},
	}, sty))

	assert.Contains(t, out, styles.CronOneShotIcon)
	assert.Contains(t, out, "2 Scheduled")
	assert.NotContains(t, out, "⏰", "the clock emoji should be gone from the pills panel")
}
