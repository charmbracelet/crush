package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/scheduler"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// cronTestNow is the reference "now" every cron rendering test formats
// against: a Wednesday, so the weekday-relative branches are exercised
// without wrapping past the week boundary.
var cronTestNow = time.Date(2026, time.July, 29, 12, 0, 0, 0, time.Local)

func cronTestTask(id, cron, prompt string, next time.Time) scheduler.Task {
	return scheduler.Task{
		ID:        id,
		SessionID: "session-1",
		Prompt:    prompt,
		Cron:      cron,
		Recurring: true,
		CreatedAt: cronTestNow,
		NextRunAt: next,
	}
}

// cronTestOpts builds the render options for a finished cron tool call
// whose result carries the given tasks as metadata.
func cronTestOpts(t *testing.T, input string, tasks []scheduler.Task) *ToolRenderOpts {
	t.Helper()
	meta, err := json.Marshal(tools.CronTasksResponseMetadata{Tasks: tasks})
	require.NoError(t, err)
	return &ToolRenderOpts{
		ToolCall: message.ToolCall{ID: "call-1", Name: tools.CronListToolName, Input: input, Finished: true},
		Result:   &message.ToolResult{Content: "ok", Metadata: string(meta)},
		Status:   ToolStatusSuccess,
	}
}

func TestFormatCronTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{
			name: "later today shows clock time",
			when: time.Date(2026, time.July, 29, 14, 30, 0, 0, time.Local),
			want: "14:30",
		},
		{
			name: "earlier today still shows clock time",
			when: time.Date(2026, time.July, 29, 9, 5, 0, 0, time.Local),
			want: "09:05",
		},
		{
			name: "within the coming week shows the weekday",
			when: time.Date(2026, time.August, 3, 9, 0, 0, 0, time.Local),
			want: "Mon 09:00",
		},
		{
			name: "within the past week shows the weekday",
			when: time.Date(2026, time.July, 27, 9, 0, 0, 0, time.Local),
			want: "Mon 09:00",
		},
		{
			name: "further out this year is dated",
			when: time.Date(2026, time.September, 1, 8, 0, 0, 0, time.Local),
			want: "Sep 1 08:00",
		},
		{
			name: "another year carries the year",
			when: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.Local),
			want: "2027-01-01 00:00",
		},
		{
			name: "zero time renders as a dash",
			when: time.Time{},
			want: "—",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, formatCronTime(tt.when, cronTestNow))
		})
	}
}

func TestCronCalendarDaysAcrossDSTSpringForward(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("timezone database unavailable: %v", err)
	}

	// 2026-03-08 is the US spring-forward day: the midnight-to-midnight
	// gap to 2026-03-09 is only 23 wall-clock hours, so hours/24 would
	// truncate tomorrow to "0 days away" and a week out to 6.
	transition := time.Date(2026, time.March, 8, 9, 0, 0, 0, loc)
	require.Equal(t, 1, cronCalendarDays(transition, time.Date(2026, time.March, 9, 15, 4, 0, 0, loc)))
	require.Equal(t, 7, cronCalendarDays(transition, time.Date(2026, time.March, 15, 9, 0, 0, 0, loc)))
	require.Equal(t, -1, cronCalendarDays(transition, time.Date(2026, time.March, 7, 9, 0, 0, 0, loc)))
	require.Equal(t, 0, cronCalendarDays(transition, time.Date(2026, time.March, 8, 23, 59, 0, 0, loc)))
}

func TestCronTasksFromResultSortsBySoonestFire(t *testing.T) {
	t.Parallel()

	late := cronTestTask("bbbbbbbb", "0 9 * * 1-5", "standup", cronTestNow.Add(4*time.Hour))
	soon := cronTestTask("aaaaaaaa", "*/15 * * * *", "deploy check", cronTestNow.Add(10*time.Minute))
	opts := cronTestOpts(t, "{}", []scheduler.Task{late, soon})

	got := cronTasksFromResult(opts)
	require.Len(t, got, 2)
	require.Equal(t, "aaaaaaaa", got[0].ID)
	require.Equal(t, "bbbbbbbb", got[1].ID)
}

func TestCronTasksFromResultWithoutMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts *ToolRenderOpts
	}{
		{
			name: "no result",
			opts: &ToolRenderOpts{ToolCall: message.ToolCall{Finished: true}},
		},
		{
			name: "empty metadata",
			opts: &ToolRenderOpts{
				ToolCall: message.ToolCall{Finished: true},
				Result:   &message.ToolResult{Content: "No scheduled tasks in this session."},
			},
		},
		{
			name: "unparsable metadata",
			opts: &ToolRenderOpts{
				ToolCall: message.ToolCall{Finished: true},
				Result:   &message.ToolResult{Content: "ok", Metadata: "not json"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Empty(t, cronTasksFromResult(tt.opts))
		})
	}
}

func TestFormatCronTasksListCollapsed(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tasks := []scheduler.Task{
		cronTestTask("aaaaaaaa", "*/15 * * * *", "Check the deploy status", cronTestNow.Add(10*time.Minute)),
		cronTestTask("bbbbbbbb", "0 9 * * 1-5", "Post the standup summary", cronTestNow.Add(21*time.Hour)),
	}

	out := ansi.Strip(formatCronTasksList(&sty, tasks, 120, cronListOpts{now: cronTestNow}))
	lines := strings.Split(out, "\n")

	require.Len(t, lines, 2, "collapsed rendering is one line per task")
	require.Contains(t, lines[0], styles.CronRecurringIcon)
	require.Contains(t, lines[0], "aaaaaaaa")
	require.Contains(t, lines[0], "*/15 * * * *")
	require.Contains(t, lines[0], "next 12:10")
	require.Contains(t, lines[0], "Check the deploy status")
	require.Contains(t, lines[1], "bbbbbbbb")
	// Collapsed rows carry no run metadata.
	require.NotContains(t, out, "never run")
}

func TestFormatCronTasksListCollapsedAlignsScheduleColumn(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tasks := []scheduler.Task{
		cronTestTask("aaaaaaaa", "0 9 * * 1-5", "short expression", cronTestNow.Add(time.Hour)),
		cronTestTask("bbbbbbbb", "*/5 0,6,12,18 * * *", "long expression", cronTestNow.Add(2*time.Hour)),
	}

	out := ansi.Strip(formatCronTasksList(&sty, tasks, 120, cronListOpts{now: cronTestNow}))
	lines := strings.Split(out, "\n")
	require.Len(t, lines, 2)

	require.Equal(t,
		strings.Index(lines[0], "next "),
		strings.Index(lines[1], "next "),
		"the next-run column lines up across rows",
	)
}

func TestFormatCronTasksListCollapsedTruncates(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	var tasks []scheduler.Task
	for i := range cronCollapsedTaskLimit + 3 {
		tasks = append(tasks, cronTestTask(
			fmt.Sprintf("task%04d", i),
			"*/15 * * * *",
			fmt.Sprintf("task %d", i),
			cronTestNow.Add(time.Duration(i)*time.Minute),
		))
	}

	out := ansi.Strip(formatCronTasksList(&sty, tasks, 120, cronListOpts{now: cronTestNow}))
	lines := strings.Split(out, "\n")

	require.Len(t, lines, cronCollapsedTaskLimit+1, "capped rows plus the truncation line")
	require.Contains(t, lines[len(lines)-1], "… and 3 more")
	require.NotContains(t, out, "task0005")
}

func TestFormatCronTasksListCollapsedFitsWidth(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tasks := []scheduler.Task{
		cronTestTask("aaaaaaaa", "*/15 * * * *", strings.Repeat("a very long prompt ", 20), cronTestNow.Add(time.Minute)),
	}

	const width = 60
	out := formatCronTasksList(&sty, tasks, width, cronListOpts{now: cronTestNow})
	for _, line := range strings.Split(out, "\n") {
		require.LessOrEqual(t, ansi.StringWidth(line), width)
	}
}

func TestFormatCronTasksListExpanded(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	lastRun := cronTestNow.Add(-15 * time.Minute)
	task := cronTestTask("aaaaaaaa", "*/15 * * * *", "Check the deploy status", cronTestNow.Add(10*time.Minute))
	task.Durable = true
	task.RunCount = 12
	task.LastRunAt = &lastRun
	task.LastError = "session no longer exists"

	out := ansi.Strip(formatCronTasksList(&sty, []scheduler.Task{task}, 120, cronListOpts{
		expanded: true,
		now:      cronTestNow,
	}))

	require.Contains(t, out, "aaaaaaaa")
	require.Contains(t, out, "next 12:10")
	require.Contains(t, out, "Check the deploy status")
	require.Contains(t, out, "recurring · durable · 12 runs · last 11:45")
	require.Contains(t, out, "error: session no longer exists")

	for _, line := range strings.Split(out, "\n")[1:] {
		require.True(t, strings.HasPrefix(line, cronDetailIndent), "detail lines are indented: %q", line)
	}
}

func TestFormatCronTasksListExpandedShowsEveryTask(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	var tasks []scheduler.Task
	for i := range cronCollapsedTaskLimit + 3 {
		tasks = append(tasks, cronTestTask(
			fmt.Sprintf("task%04d", i),
			"*/15 * * * *",
			fmt.Sprintf("task %d", i),
			cronTestNow.Add(time.Duration(i)*time.Minute),
		))
	}

	out := ansi.Strip(formatCronTasksList(&sty, tasks, 120, cronListOpts{expanded: true, now: cronTestNow}))

	for i := range tasks {
		require.Contains(t, out, fmt.Sprintf("task%04d", i))
	}
	require.NotContains(t, out, "and 3 more")
}

func TestFormatCronTasksListOneShotAndDeleted(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	oneShot := cronTestTask("aaaaaaaa", "30 14 29 7 *", "Remind me to review the PR", cronTestNow.Add(2*time.Hour))
	oneShot.Recurring = false

	out := ansi.Strip(formatCronTasksList(&sty, []scheduler.Task{oneShot}, 120, cronListOpts{
		expanded: true,
		now:      cronTestNow,
	}))
	require.Contains(t, out, "one-shot · session-only · never run")
	require.NotContains(t, out, styles.CronRecurringIcon)

	deleted := ansi.Strip(formatCronTasksList(&sty, []scheduler.Task{oneShot}, 120, cronListOpts{
		deleted: true,
		now:     cronTestNow,
	}))
	require.Contains(t, deleted, styles.CronDeletedIcon)
	require.Contains(t, deleted, "cancelled")
	require.NotContains(t, deleted, "next ")
}

func TestFormatCronTasksListNoNextRun(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	task := cronTestTask("aaaaaaaa", "30 14 29 7 *", "one and done", time.Time{})
	task.Recurring = false

	out := ansi.Strip(formatCronTasksList(&sty, []scheduler.Task{task}, 120, cronListOpts{now: cronTestNow}))
	require.Contains(t, out, "no next run")
}

func TestCronHeaderParams(t *testing.T) {
	t.Parallel()

	task := cronTestTask("aaaaaaaa", "*/15 * * * *", "Check the deploy status", cronTestNow.Add(10*time.Minute))

	tests := []struct {
		name  string
		tool  string
		input string
		tasks []scheduler.Task
		want  []string
	}{
		{
			name:  "create with a result",
			tool:  tools.CronCreateToolName,
			input: `{"cron":"*/15 * * * *","prompt":"Check the deploy status"}`,
			tasks: []scheduler.Task{task},
			want:  []string{"*/15 * * * *", "next", "12:10", "id", "aaaaaaaa"},
		},
		{
			name:  "create falls back to its input",
			tool:  tools.CronCreateToolName,
			input: `{"cron":"*/15 * * * *","prompt":"Check the deploy status"}`,
			want:  []string{"*/15 * * * *"},
		},
		{
			name:  "delete with a result",
			tool:  tools.CronDeleteToolName,
			input: `{"id":"aaaaaaaa"}`,
			tasks: []scheduler.Task{task},
			want:  []string{"aaaaaaaa", "was", "*/15 * * * *"},
		},
		{
			name:  "delete falls back to its input",
			tool:  tools.CronDeleteToolName,
			input: `{"id":"aaaaaaaa"}`,
			want:  []string{"aaaaaaaa"},
		},
		{
			name:  "list pluralizes",
			tool:  tools.CronListToolName,
			input: "{}",
			tasks: []scheduler.Task{task, cronTestTask("bbbbbbbb", "0 9 * * *", "standup", cronTestNow.Add(21*time.Hour))},
			want:  []string{"2 tasks", "next", "12:10"},
		},
		{
			name:  "list with a single task",
			tool:  tools.CronListToolName,
			input: "{}",
			tasks: []scheduler.Task{task},
			want:  []string{"1 task", "next", "12:10"},
		},
		{
			name:  "list with no tasks",
			tool:  tools.CronListToolName,
			input: "{}",
			want:  []string{"no tasks"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := &CronToolRenderContext{tool: tt.tool}
			opts := &ToolRenderOpts{ToolCall: message.ToolCall{Input: tt.input, Finished: true}}
			require.Equal(t, tt.want, ctx.headerParams(opts, tt.tasks, cronTestNow))
		})
	}
}

func TestCronRenderToolBody(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &CronToolRenderContext{tool: tools.CronListToolName}
	opts := cronTestOpts(t, "{}", []scheduler.Task{
		cronTestTask("aaaaaaaa", "*/15 * * * *", "Check the deploy status", cronTestNow.Add(10*time.Minute)),
	})

	out := ansi.Strip(ctx.RenderTool(&sty, 120, opts))
	require.Contains(t, out, "Scheduled")
	require.Contains(t, out, "1 task")
	require.Contains(t, out, "aaaaaaaa")
	require.Contains(t, out, "Check the deploy status")
	// The tool's own prose is replaced by the rendered line items.
	require.NotContains(t, out, "ok")
}

func TestCronRenderToolFallsBackToResultContent(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &CronToolRenderContext{tool: tools.CronListToolName}
	opts := &ToolRenderOpts{
		ToolCall: message.ToolCall{ID: "call-1", Name: tools.CronListToolName, Input: "{}", Finished: true},
		Result:   &message.ToolResult{Content: "No scheduled tasks in this session."},
		Status:   ToolStatusSuccess,
	}

	out := ansi.Strip(ctx.RenderTool(&sty, 120, opts))
	require.Contains(t, out, "no tasks")
	require.Contains(t, out, "No scheduled tasks in this session.")
}

func TestCronRenderToolStates(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &CronToolRenderContext{tool: tools.CronCreateToolName}
	input := `{"cron":"*/15 * * * *","prompt":"Check the deploy status"}`

	pending := ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: message.ToolCall{ID: "call-1", Name: tools.CronCreateToolName, Input: input},
		Status:   ToolStatusRunning,
	}))
	require.Contains(t, pending, "Schedule")

	errored := ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: message.ToolCall{ID: "call-1", Name: tools.CronCreateToolName, Input: input, Finished: true},
		Result:   &message.ToolResult{Content: "cron expression is valid but will never fire", IsError: true},
		Status:   ToolStatusError,
	}))
	require.Contains(t, errored, "ERROR")
	require.Contains(t, errored, "never fire")

	compact := ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: message.ToolCall{ID: "call-1", Name: tools.CronCreateToolName, Input: input, Finished: true},
		Result:   &message.ToolResult{Content: "ok"},
		Status:   ToolStatusSuccess,
		Compact:  true,
	}))
	require.Equal(t, 1, len(strings.Split(compact, "\n")), "compact mode is header-only")
	require.Contains(t, compact, "*/15 * * * *")
}

func TestNewToolMessageItemRoutesCronTools(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	for _, name := range []string{tools.CronCreateToolName, tools.CronListToolName, tools.CronDeleteToolName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{ID: "call-1", Name: name, Input: "{}"}, nil, false)
			require.IsType(t, &baseToolMessageItem{}, item)
			base, ok := item.(*baseToolMessageItem)
			require.True(t, ok)
			renderer, ok := base.toolRenderer.(*CronToolRenderContext)
			require.True(t, ok, "cron tools use the cron renderer")
			require.Equal(t, name, renderer.tool)
		})
	}
}
