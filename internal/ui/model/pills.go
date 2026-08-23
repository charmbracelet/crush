package model

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/scheduler"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

const (
	// pillHeightWithBorder is the height of a pill including its border.
	pillHeightWithBorder = 3
	// maxTaskDisplayLength is the maximum length of a task name in the pill.
	maxTaskDisplayLength = 40
	// maxQueueDisplayLength is the maximum length of a queue item in the list.
	maxQueueDisplayLength = 60
)

// hasIncompleteTodos returns true if there are any non-completed todos.
func hasIncompleteTodos(todos []session.Todo) bool {
	return session.HasIncompleteTodos(todos)
}

// hasInProgressTodo returns true if there is at least one in-progress todo.
func hasInProgressTodo(todos []session.Todo) bool {
	for _, todo := range todos {
		if todo.Status == session.TodoStatusInProgress {
			return true
		}
	}
	return false
}

// queuePill renders the queue count pill with gradient triangles. Pills always
// render with a border; focus within the expanded panel is conveyed by the list
// shown below the pills, not by hiding a pill's border.
func queuePill(queue int, t *styles.Styles) string {
	if queue <= 0 {
		return ""
	}
	triangles := styles.ForegroundGrad(t.Pills.QueueIconBase, "▶▶▶▶▶▶▶▶▶", false, t.Pills.QueueGradFromColor, t.Pills.QueueGradToColor)
	if queue < len(triangles) {
		triangles = triangles[:queue]
	}

	text := t.Pills.QueueLabel.Render(fmt.Sprintf("%d Queued", queue))
	content := fmt.Sprintf("%s %s", strings.Join(triangles, ""), text)
	return t.Pills.Focused.Render(content)
}

// todoPill renders the todo progress pill with optional spinner and task name.
// When the panel is expanded the current task is omitted from the pill, since
// the list below already shows it.
func todoPill(todos []session.Todo, spinnerView string, expanded bool, t *styles.Styles) string {
	if !hasIncompleteTodos(todos) {
		return ""
	}

	completed := 0
	var currentTodo *session.Todo
	for i := range todos {
		switch todos[i].Status {
		case session.TodoStatusCompleted:
			completed++
		case session.TodoStatusInProgress:
			if currentTodo == nil {
				currentTodo = &todos[i]
			}
		}
	}

	total := len(todos)

	label := t.Pills.TodoLabel.Render("To-Do")
	progress := t.Pills.TodoProgress.Render(fmt.Sprintf("%d/%d", completed, total))

	var content string
	if expanded {
		content = fmt.Sprintf("%s %s", label, progress)
	} else if currentTodo != nil {
		taskText := currentTodo.Content
		if currentTodo.ActiveForm != "" {
			taskText = currentTodo.ActiveForm
		}
		if ansi.StringWidth(taskText) > maxTaskDisplayLength {
			taskText = ansi.Truncate(taskText, maxTaskDisplayLength-1, "…")
		}
		task := t.Pills.TodoCurrentTask.Render(taskText)
		content = fmt.Sprintf("%s %s %s  %s", spinnerView, label, progress, task)
	} else {
		content = fmt.Sprintf("%s %s", label, progress)
	}

	return t.Pills.Focused.Render(content)
}

// todoList renders the expanded todo list.
func todoList(sessionTodos []session.Todo, spinnerView string, t *styles.Styles, width int) string {
	return chat.FormatTodosList(t, sessionTodos, spinnerView, width)
}

// queueList renders the expanded queue items list.
func queueList(queueItems []string, t *styles.Styles) string {
	if len(queueItems) == 0 {
		return ""
	}

	var lines []string
	for _, item := range queueItems {
		text := item
		if ansi.StringWidth(text) > maxQueueDisplayLength {
			text = ansi.Truncate(text, maxQueueDisplayLength-1, "…")
		}
		prefix := t.Pills.QueueItemPrefix.Render() + " "
		lines = append(lines, prefix+t.Pills.QueueItemText.Render(text))
	}

	return strings.Join(lines, "\n")
}

// cronPill renders the scheduled-task count pill. The glyph matches the
// stopwatch used by the harness TUI for scheduled entries, not a clock
// emoji — keeping the pills panel visually consistent with the rest of
// the TUI's geometric icon set.
func cronPill(tasks []scheduler.Task, t *styles.Styles) string {
	if len(tasks) == 0 {
		return ""
	}
	icon := t.Pills.QueueLabel.Render(styles.CronOneShotIcon)
	label := t.Pills.QueueLabel.Render(fmt.Sprintf("%d Scheduled", len(tasks)))
	content := fmt.Sprintf("%s %s", icon, label)
	return t.Pills.Focused.Render(content)
}

// cronList renders the expanded scheduled-task list. Each item's prefix
// glyph distinguishes recurring (↻) from one-shot (⏱) tasks so the
// recurrence state is legible at a glance without expanding the full
// tool-call body.
func cronList(tasks []scheduler.Task, t *styles.Styles) string {
	if len(tasks) == 0 {
		return ""
	}

	var lines []string
	for _, task := range tasks {
		nextRun := task.NextRunAt.Local().Format("15:04")
		if task.NextRunAt.Local().Day() != time.Now().Local().Day() {
			nextRun = task.NextRunAt.Local().Format("Jan 2 15:04")
		}
		prompt := task.Prompt
		if ansi.StringWidth(prompt) > maxQueueDisplayLength {
			prompt = ansi.Truncate(prompt, maxQueueDisplayLength-1, "…")
		}
		// Recurrence is carried by the prefix glyph; repeating it inline
		// would print ↻ twice on the same row.
		text := fmt.Sprintf("%s %s — %s", task.ID, nextRun, prompt)
		// The prefix glyph distinguishes recurring from one-shot tasks,
		// keeping scheduled items visually distinct from the queued-prompt
		// list directly above them when both sections are expanded.
		prefix := cronItemPrefix(task, t) + " "
		lines = append(lines, prefix+t.Pills.QueueItemText.Render(text))
	}

	return strings.Join(lines, "\n")
}

// cronItemPrefix returns the glyph prefix for a scheduled-task list
// item: ↻ for recurring, ⏱ for one-shot.
func cronItemPrefix(task scheduler.Task, t *styles.Styles) string {
	if task.Recurring {
		// Tool.CronRecurringIcon carries no SetString — unlike the Pills
		// styles, which do — so the glyph has to be passed in. Rendering
		// it with no argument styles the empty string and the row loses
		// its prefix entirely.
		return t.Tool.CronRecurringIcon.Render(styles.CronRecurringIcon)
	}
	return t.Pills.CronItemPrefix.Render()
}

// pillsHeightReasonableTerminalHeight is the minimum terminal height at which
// we auto-expand pills when there are incomplete todos.
const pillsHeightReasonableTerminalHeight = 40

// autoExpandPillsIfReasonable expands the pills panel if the terminal has
// enough vertical space to show the expanded list comfortably.
func (m *UI) autoExpandPillsIfReasonable() tea.Cmd {
	if !m.hasSession() {
		return nil
	}
	if m.activeInline != nil {
		return nil
	}
	if m.height < pillsHeightReasonableTerminalHeight {
		return nil
	}
	hasPills := hasIncompleteTodos(m.session.Todos) || m.promptQueue > 0 || len(m.cronTasks) > 0
	if !hasPills {
		return nil
	}
	if m.pillsExpanded {
		return nil
	}
	if m.pillsAutoExpanded {
		return nil
	}
	m.pillsExpanded = true
	m.pillsAutoExpanded = true
	m.updateLayoutAndSize()
	if m.chat.Follow() {
		m.chat.ScrollToBottom()
	}
	return nil
}

// togglePillsExpanded toggles the pills panel expansion state.
func (m *UI) togglePillsExpanded() tea.Cmd {
	if !m.hasSession() {
		return nil
	}
	hasPills := hasIncompleteTodos(m.session.Todos) || m.promptQueue > 0 || len(m.cronTasks) > 0
	if !hasPills {
		return nil
	}
	m.pillsExpanded = !m.pillsExpanded
	m.updateLayoutAndSize()

	// Make sure to follow scroll if follow is enabled when toggling pills.
	// Note: uses ScrollToBottom (no scrollbar) since this is layout adjustment,
	// not user-initiated scrolling.
	if m.chat.Follow() {
		m.chat.ScrollToBottom()
	}

	return nil
}

// pillsAreaHeight calculates the total height needed for the pills area.
func (m *UI) pillsAreaHeight() int {
	if !m.hasSession() {
		return 0
	}
	// Suppress pills when an inline editor (e.g. question form) is active
	// to avoid competing for screen space.
	if m.activeInline != nil {
		return 0
	}
	hasIncomplete := hasIncompleteTodos(m.session.Todos)
	hasQueue := m.promptQueue > 0
	hasCron := len(m.cronTasks) > 0
	hasPills := hasIncomplete || hasQueue || hasCron
	if !hasPills {
		return 0
	}

	pillsAreaHeight := pillHeightWithBorder
	if m.pillsExpanded {
		// Every section with content expands, so the panel needs room for all
		// of their lists stacked, not just one.
		if hasIncomplete {
			pillsAreaHeight += len(m.session.Todos)
		}
		if hasQueue {
			pillsAreaHeight += m.promptQueue
		}
		if hasCron {
			pillsAreaHeight += len(m.cronTasks)
		}
	}
	return pillsAreaHeight
}

// renderPills renders the pills panel and stores it in m.pillsView.
func (m *UI) renderPills() {
	m.pillsView = ""
	if !m.hasSession() {
		return
	}
	// Suppress pills when an inline editor (e.g. question form) is active.
	if m.activeInline != nil {
		return
	}

	width := m.layout.pills.Dx()
	if width <= 0 {
		return
	}

	paddingLeft := 3
	contentWidth := max(width-paddingLeft, 0)

	hasIncomplete := hasIncompleteTodos(m.session.Todos)
	hasQueue := m.promptQueue > 0
	hasCron := len(m.cronTasks) > 0

	if !hasIncomplete && !hasQueue && !hasCron {
		return
	}

	t := m.com.Styles

	inProgressIcon := t.Tool.TodoInProgressIcon.Render(styles.SpinnerIcon)
	if m.todoIsSpinning {
		inProgressIcon = m.todoSpinner.View()
	}

	var pills []string
	if hasIncomplete {
		pills = append(pills, todoPill(m.session.Todos, inProgressIcon, m.pillsExpanded, t))
	}
	if hasQueue {
		pills = append(pills, queuePill(m.promptQueue, t))
	}
	if hasCron {
		pills = append(pills, cronPill(m.cronTasks, t))
	}

	// Expanding shows every section that has content, stacked in the same order
	// as the pills above them, so the panel matches what the pills advertise.
	var expandedSections []string
	if m.pillsExpanded {
		if hasIncomplete {
			expandedSections = append(expandedSections, todoList(m.session.Todos, inProgressIcon, t, contentWidth))
		}
		// Render from the memoized queue (fetched off-thread, see
		// workspace_cache.go): renderPills runs on the Update/View
		// path and must never block on a workspace round-trip.
		if hasQueue && len(m.promptQueueItems) > 0 {
			expandedSections = append(expandedSections, queueList(m.promptQueueItems, t))
		}
		if hasCron {
			expandedSections = append(expandedSections, cronList(m.cronTasks, t))
		}
	}
	expandedList := lipgloss.JoinVertical(lipgloss.Left, expandedSections...)

	if len(pills) == 0 {
		return
	}

	pillsRow := lipgloss.JoinHorizontal(lipgloss.Top, pills...)

	helpDesc := "open"
	if m.pillsExpanded {
		helpDesc = "close"
	}
	helpKey := t.Pills.HelpKey.Render("ctrl+t")
	helpText := t.Pills.HelpText.Render(helpDesc)
	helpHint := lipgloss.JoinHorizontal(lipgloss.Center, helpKey, " ", helpText)
	pillsRow = lipgloss.JoinHorizontal(lipgloss.Center, pillsRow, " ", helpHint)

	pillsArea := pillsRow
	if expandedList != "" {
		pillsArea = lipgloss.JoinVertical(lipgloss.Left, pillsRow, expandedList)
	}

	m.pillsView = t.Pills.Area.MaxWidth(width).PaddingLeft(paddingLeft).Render(pillsArea)
}
