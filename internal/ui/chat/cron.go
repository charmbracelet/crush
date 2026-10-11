package chat

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/scheduler"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// -----------------------------------------------------------------------------
// Cron Tools (CronCreate, CronList, CronDelete)
// -----------------------------------------------------------------------------

const (
	// cronCollapsedTaskLimit caps how many scheduled tasks a collapsed
	// body lists before it truncates. A session may hold up to
	// [scheduler.MaxTasksPerSession] tasks, which would otherwise bury
	// the rest of the conversation on a single CronList call.
	cronCollapsedTaskLimit = 5

	// cronScheduleColumnWidth caps the width of the cron-expression
	// column. Expressions are padded to the widest one on display so the
	// "next" column lines up; a pathological list of comma expressions
	// would otherwise push the prompt off the right edge.
	cronScheduleColumnWidth = 22

	// cronDetailIndent is the indent applied to a task's prompt and
	// metadata lines when the tool call is expanded.
	cronDetailIndent = "  "
)

// CronToolMessageItem is a message item that represents a scheduled-task
// tool call: CronCreate, CronList, or CronDelete.
type CronToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*CronToolMessageItem)(nil)

// NewCronCreateToolMessageItem creates a [CronToolMessageItem] for CronCreate.
func NewCronCreateToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &CronToolRenderContext{tool: tools.CronCreateToolName}, canceled)
}

// NewCronListToolMessageItem creates a [CronToolMessageItem] for CronList.
func NewCronListToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &CronToolRenderContext{tool: tools.CronListToolName}, canceled)
}

// NewCronDeleteToolMessageItem creates a [CronToolMessageItem] for CronDelete.
func NewCronDeleteToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &CronToolRenderContext{tool: tools.CronDeleteToolName}, canceled)
}

// CronToolRenderContext renders scheduled-task tool messages. The three
// cron tools share a body renderer — a list of scheduled-task line items
// — and differ only in their header and in whether the tasks they carry
// are live or just-cancelled.
type CronToolRenderContext struct {
	tool string
}

// cronToolLabel returns the header label for a cron tool.
func cronToolLabel(tool string) string {
	switch tool {
	case tools.CronCreateToolName:
		return "Schedule"
	case tools.CronDeleteToolName:
		return "Unschedule"
	default:
		return "Scheduled"
	}
}

// RenderTool implements the [ToolRenderer] interface.
func (c *CronToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	label := cronToolLabel(c.tool)
	if opts.IsPending() {
		return pendingTool(sty, label, opts.Anim, opts.Compact)
	}

	now := time.Now()
	tasks := cronTasksFromResult(opts)
	header := toolHeader(sty, opts.Status, label, cappedWidth, opts, c.headerParams(opts, tasks, now)...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.HasEmptyResult() {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal

	// No metadata means either an empty CronList or a result written
	// before this renderer existed. Either way the tool's own prose is
	// the best available body.
	if len(tasks) == 0 {
		return joinToolParts(header, sty.Tool.Body.Render(
			toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent),
		))
	}

	body := formatCronTasksList(sty, tasks, bodyWidth, cronListOpts{
		expanded: opts.ExpandedContent,
		deleted:  c.tool == tools.CronDeleteToolName,
		now:      now,
	})
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

// headerParams builds the tool-header parameters for this cron tool: a
// main parameter plus alternating key/value pairs, per [toolHeader].
// Tasks come from the result metadata when the call finished; before
// that (and when metadata is missing) the call's own input is used, so
// the header still says something useful.
func (c *CronToolRenderContext) headerParams(opts *ToolRenderOpts, tasks []scheduler.Task, now time.Time) []string {
	switch c.tool {
	case tools.CronCreateToolName:
		var params tools.CronCreateParams
		_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

		schedule := params.Cron
		if len(tasks) > 0 {
			schedule = tasks[0].Cron
		}
		if schedule == "" {
			schedule = "scheduled task"
		}

		toolParams := []string{schedule}
		if len(tasks) > 0 {
			toolParams = append(toolParams,
				"next", formatCronTime(tasks[0].NextRunAt, now),
				"id", tasks[0].ID,
			)
		}
		return toolParams

	case tools.CronDeleteToolName:
		var params tools.CronDeleteParams
		_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

		id := params.ID
		if len(tasks) > 0 {
			id = tasks[0].ID
		}
		if id == "" {
			id = "scheduled task"
		}

		toolParams := []string{id}
		if len(tasks) > 0 {
			toolParams = append(toolParams, "was", tasks[0].Cron)
		}
		return toolParams

	default: // CronList
		if len(tasks) == 0 {
			return []string{"no tasks"}
		}
		main := fmt.Sprintf("%d tasks", len(tasks))
		if len(tasks) == 1 {
			main = "1 task"
		}
		// tasks is sorted by next run, so the head is the soonest fire.
		return []string{main, "next", formatCronTime(tasks[0].NextRunAt, now)}
	}
}

// cronTasksFromResult pulls the scheduled tasks a cron tool attached to
// its result, sorted soonest-fire first. It returns nil when the result
// carries no task metadata.
func cronTasksFromResult(opts *ToolRenderOpts) []scheduler.Task {
	if !opts.HasResult() || opts.Result.Metadata == "" {
		return nil
	}
	var meta tools.CronTasksResponseMetadata
	if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err != nil {
		return nil
	}
	tasks := slices.Clone(meta.Tasks)
	slices.SortStableFunc(tasks, func(a, b scheduler.Task) int {
		return a.NextRunAt.Compare(b.NextRunAt)
	})
	return tasks
}

// cronListOpts controls how a scheduled-task list renders.
type cronListOpts struct {
	// expanded shows every task, each with its full prompt and run
	// metadata, instead of one truncated line per task.
	expanded bool
	// deleted renders the tasks as cancelled rather than live.
	deleted bool
	// now is the reference time the "next run" column is formatted
	// against.
	now time.Time
}

// formatCronTasksList renders scheduled tasks as line items.
//
// Collapsed, each task is a single line — icon, ID, schedule, next fire,
// prompt — capped at [cronCollapsedTaskLimit] rows. Expanded, every task
// is listed with its prompt wrapped onto its own lines plus a metadata
// line carrying recurrence, durability, run count, last run, and the
// last error if the task has one.
func formatCronTasksList(sty *styles.Styles, tasks []scheduler.Task, width int, opts cronListOpts) string {
	if len(tasks) == 0 {
		return ""
	}

	shown := tasks
	hidden := 0
	if !opts.expanded && len(tasks) > cronCollapsedTaskLimit {
		shown = tasks[:cronCollapsedTaskLimit]
		hidden = len(tasks) - cronCollapsedTaskLimit
	}

	// Pad the schedule column to the widest expression on display so the
	// columns to its right line up.
	scheduleWidth := 0
	for _, task := range shown {
		scheduleWidth = max(scheduleWidth, ansi.StringWidth(task.Cron))
	}
	scheduleWidth = min(scheduleWidth, cronScheduleColumnWidth)

	var lines []string
	for i, task := range shown {
		if opts.expanded && i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, cronTaskLines(sty, task, width, scheduleWidth, opts)...)
	}

	if hidden > 0 {
		lines = append(lines, sty.Tool.ResultTruncation.Render(fmt.Sprintf("… and %d more", hidden)))
	}

	return strings.Join(lines, "\n")
}

// cronTaskLines renders one scheduled task: its header row, plus — when
// expanded — the wrapped prompt and a metadata line.
func cronTaskLines(sty *styles.Styles, task scheduler.Task, width, scheduleWidth int, opts cronListOpts) []string {
	icon := sty.Tool.CronRecurringIcon.Render(styles.CronRecurringIcon)
	switch {
	case opts.deleted:
		icon = sty.Tool.CronDeletedIcon.Render(styles.CronDeletedIcon)
	case !task.Recurring:
		icon = sty.Tool.CronOneShotIcon.Render(styles.CronOneShotIcon)
	}

	schedule := ansi.Truncate(task.Cron, scheduleWidth, "…")
	if pad := scheduleWidth - ansi.StringWidth(schedule); pad > 0 {
		schedule += strings.Repeat(" ", pad)
	}

	row := fmt.Sprintf("%s %s  %s  %s",
		icon,
		sty.Tool.CronTaskID.Render(task.ID),
		sty.Tool.CronSchedule.Render(schedule),
		sty.Tool.CronNextRun.Render(cronNextRunLabel(task, opts)),
	)

	if !opts.expanded {
		// Collapsed, the prompt shares the row and is cut to fit.
		row += "  " + sty.Tool.CronPrompt.Render(task.Prompt)
		return []string{ansi.Truncate(row, width, "…")}
	}

	lines := []string{ansi.Truncate(row, width, "…")}
	if task.Prompt != "" {
		lines = append(lines, indentCronDetail(sty.Tool.CronPrompt.Render(task.Prompt), width)...)
	}
	lines = append(lines, indentCronDetail(sty.Tool.CronMeta.Render(cronTaskMeta(task, opts)), width)...)
	if task.LastError != "" {
		lines = append(lines, indentCronDetail(
			sty.Tool.CronError.Render("error: "+strings.ReplaceAll(task.LastError, "\n", " ")),
			width,
		)...)
	}
	return lines
}

// cronNextRunLabel renders a task's next-fire column. A cancelled task
// has no next fire, and neither does a one-shot that already ran.
func cronNextRunLabel(task scheduler.Task, opts cronListOpts) string {
	if opts.deleted {
		return "cancelled"
	}
	if task.NextRunAt.IsZero() {
		return "no next run"
	}
	return "next " + formatCronTime(task.NextRunAt, opts.now)
}

// cronTaskMeta builds a task's detail line: recurrence, durability, how
// many times it has fired, and when it last did.
func cronTaskMeta(task scheduler.Task, opts cronListOpts) string {
	parts := []string{"recurring"}
	if !task.Recurring {
		parts = []string{"one-shot"}
	}
	if task.Durable {
		parts = append(parts, "durable")
	} else {
		parts = append(parts, "session-only")
	}
	switch task.RunCount {
	case 0:
		parts = append(parts, "never run")
	case 1:
		parts = append(parts, "1 run")
	default:
		parts = append(parts, fmt.Sprintf("%d runs", task.RunCount))
	}
	if task.LastRunAt != nil && !task.LastRunAt.IsZero() {
		parts = append(parts, "last "+formatCronTime(*task.LastRunAt, opts.now))
	}
	return strings.Join(parts, " · ")
}

// indentCronDetail wraps a detail line to the available width and
// indents every resulting line under the task's header row.
func indentCronDetail(text string, width int) []string {
	available := max(width-len(cronDetailIndent), 1)
	wrapped := ansi.Hardwrap(text, available, false)
	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		lines[i] = cronDetailIndent + line
	}
	return lines
}

// formatCronTime renders a scheduled-task timestamp relative to now:
// clock time today, weekday and clock time within the coming week, and
// a dated form beyond that. Times are always shown in local time, which
// is the timezone cron expressions are evaluated in.
func formatCronTime(t, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	t = t.Local()
	now = now.Local()

	days := cronCalendarDays(now, t)

	switch {
	case days == 0:
		return t.Format("15:04")
	case days > 0 && days < 7:
		return t.Format("Mon 15:04")
	case days < 0 && days > -7:
		return t.Format("Mon 15:04")
	case t.Year() == now.Year():
		return t.Format("Jan 2 15:04")
	default:
		return t.Format("2006-01-02 15:04")
	}
}

// cronCalendarDays returns the number of calendar days from a's date to
// b's date, ignoring clock times. The midnights are compared in UTC so
// the count is unaffected by DST transitions between them: dividing
// wall-clock hours by 24 undercounts across spring-forward, where a
// midnight-to-midnight gap is only 23 hours.
func cronCalendarDays(a, b time.Time) int {
	from := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	to := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(to.Sub(from).Hours() / 24)
}
