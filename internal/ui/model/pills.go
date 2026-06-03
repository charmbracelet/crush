package model

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/goal"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// pillStyle returns the appropriate style for a pill based on focus state.
func pillStyle(focused, panelFocused bool, t *styles.Styles) lipgloss.Style {
	if !panelFocused || focused {
		return t.Pills.Focused
	}
	return t.Pills.Blurred
}

const (
	// pillHeightWithBorder is the height of a pill including its border.
	pillHeightWithBorder = 3
	// maxTaskDisplayLength is the maximum length of a task name in the pill.
	maxTaskDisplayLength = 40
	// maxQueueDisplayLength is the maximum length of a queue item in the list.
	maxQueueDisplayLength = 60
)

// pillSection represents which section of the pills panel is focused.
type pillSection int

const (
	pillSectionGoal pillSection = iota
	pillSectionTodos
	pillSectionQueue
)

// hasInProgressGoal returns true if there is an active goal.
func hasInProgressGoal(g *goal.Goal) bool {
	return g != nil && g.Status == goal.GoalActive
}

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

// goalPill renders the goal status pill.
func goalPill(g *goal.Goal, spinnerView string, focused, panelFocused bool, t *styles.Styles) string {
	if g == nil {
		return ""
	}

	label := t.Pills.TodoLabel.Render("Goal")
	status := string(g.Status)
	progress := t.Pills.TodoProgress.Render(status)

	var elapsedSeconds int64
	if g.Status == goal.GoalActive {
		elapsedSeconds = g.ActiveSeconds + int64(time.Since(g.UpdatedAt).Seconds())
	} else {
		elapsedSeconds = g.ActiveSeconds
	}
	elapsed := t.Pills.GoalElapsedTime.Render((time.Duration(elapsedSeconds) * time.Second).String())

	var content string
	if panelFocused {
		content = fmt.Sprintf("%s %s %s", label, progress, elapsed)
	} else {
		taskText := g.Objective
		if len(taskText) > maxTaskDisplayLength {
			taskText = taskText[:maxTaskDisplayLength-1] + "…"
		}
		task := t.Pills.TodoCurrentTask.Render(taskText)
		if g.Status == goal.GoalActive {
			content = fmt.Sprintf("%s %s %s %s %s", spinnerView, label, progress, task, elapsed)
		} else {
			content = fmt.Sprintf("%s %s %s %s", label, progress, task, elapsed)
		}
	}

	return pillStyle(focused, panelFocused, t).Render(content)
}

// queuePill renders the queue count pill with gradient triangles.
func queuePill(queue int, focused, panelFocused bool, t *styles.Styles) string {
	if queue <= 0 {
		return ""
	}
	triangles := styles.ForegroundGrad(t.Pills.QueueIconBase, "▶▶▶▶▶▶▶▶▶", false, t.Pills.QueueGradFromColor, t.Pills.QueueGradToColor)
	if queue < len(triangles) {
		triangles = triangles[:queue]
	}

	text := t.Pills.QueueLabel.Render(fmt.Sprintf("%d Queued", queue))
	content := fmt.Sprintf("%s %s", strings.Join(triangles, ""), text)
	return pillStyle(focused, panelFocused, t).Render(content)
}

// todoPill renders the todo progress pill with optional spinner and task name.
func todoPill(todos []session.Todo, spinnerView string, focused, panelFocused bool, t *styles.Styles) string {
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
	if panelFocused {
		content = fmt.Sprintf("%s %s", label, progress)
	} else if currentTodo != nil {
		taskText := currentTodo.Content
		if currentTodo.ActiveForm != "" {
			taskText = currentTodo.ActiveForm
		}
		if len(taskText) > maxTaskDisplayLength {
			taskText = taskText[:maxTaskDisplayLength-1] + "…"
		}
		task := t.Pills.TodoCurrentTask.Render(taskText)
		content = fmt.Sprintf("%s %s %s  %s", spinnerView, label, progress, task)
	} else {
		content = fmt.Sprintf("%s %s", label, progress)
	}

	return pillStyle(focused, panelFocused, t).Render(content)
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
		if len(text) > maxQueueDisplayLength {
			text = text[:maxQueueDisplayLength-1] + "…"
		}
		prefix := t.Pills.QueueItemPrefix.Render() + " "
		lines = append(lines, prefix+t.Pills.QueueItemText.Render(text))
	}

	return strings.Join(lines, "\n")
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
	if m.height < pillsHeightReasonableTerminalHeight {
		return nil
	}
	hasPills := hasIncompleteTodos(m.session.Todos) || m.promptQueue > 0
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
	if hasIncompleteTodos(m.session.Todos) {
		m.focusedPillSection = pillSectionTodos
	} else {
		m.focusedPillSection = pillSectionQueue
	}
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
	hasPills := hasIncompleteTodos(m.session.Todos) || m.promptQueue > 0 || m.currentGoal != nil
	if !hasPills {
		return nil
	}
	m.pillsExpanded = !m.pillsExpanded
	if m.pillsExpanded {
		if m.currentGoal != nil {
			m.focusedPillSection = pillSectionGoal
		} else if hasIncompleteTodos(m.session.Todos) {
			m.focusedPillSection = pillSectionTodos
		} else {
			m.focusedPillSection = pillSectionQueue
		}
	}
	m.updateLayoutAndSize()

	// Make sure to follow scroll if follow is enabled when toggling pills.
	if m.chat.Follow() {
		m.chat.ScrollToBottom()
	}

	return nil
}

// switchPillSection changes focus between goal, todo and queue sections.
func (m *UI) switchPillSection(dir int) tea.Cmd {
	if !m.pillsExpanded || !m.hasSession() {
		return nil
	}
	hasGoal := m.currentGoal != nil
	hasIncompleteTodos := hasIncompleteTodos(m.session.Todos)
	hasQueue := m.promptQueue > 0

	var sections []pillSection
	if hasGoal {
		sections = append(sections, pillSectionGoal)
	}
	if hasIncompleteTodos {
		sections = append(sections, pillSectionTodos)
	}
	if hasQueue {
		sections = append(sections, pillSectionQueue)
	}

	if len(sections) <= 1 {
		return nil
	}

	currentIndex := -1
	for i, s := range sections {
		if s == m.focusedPillSection {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return nil
	}

	nextIndex := (currentIndex + dir + len(sections)) % len(sections)
	m.focusedPillSection = sections[nextIndex]
	m.updateLayoutAndSize()
	return nil
}

// pillsAreaHeight calculates the total height needed for the pills area.
func (m *UI) pillsAreaHeight() int {
	if !m.hasSession() {
		return 0
	}
	hasIncomplete := hasIncompleteTodos(m.session.Todos)
	hasQueue := m.promptQueue > 0
	hasGoal := m.currentGoal != nil
	hasPills := hasIncomplete || hasQueue || hasGoal
	if !hasPills {
		return 0
	}

	pillsAreaHeight := pillHeightWithBorder
	if m.pillsExpanded {
		if m.focusedPillSection == pillSectionTodos && hasIncomplete {
			pillsAreaHeight += len(m.session.Todos)
		} else if m.focusedPillSection == pillSectionQueue && hasQueue {
			pillsAreaHeight += m.promptQueue
		} else if m.focusedPillSection == pillSectionGoal && hasGoal {
			pillsAreaHeight += 1
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

	width := m.layout.pills.Dx()
	if width <= 0 {
		return
	}

	paddingLeft := 3
	contentWidth := max(width-paddingLeft, 0)

	hasIncomplete := hasIncompleteTodos(m.session.Todos)
	hasQueue := m.promptQueue > 0
	hasGoal := m.currentGoal != nil

	if !hasIncomplete && !hasQueue && !hasGoal {
		return
	}

	t := m.com.Styles
	goalFocused := m.pillsExpanded && m.focusedPillSection == pillSectionGoal
	todosFocused := m.pillsExpanded && m.focusedPillSection == pillSectionTodos
	queueFocused := m.pillsExpanded && m.focusedPillSection == pillSectionQueue

	inProgressIcon := t.Tool.TodoInProgressIcon.Render(styles.SpinnerIcon)
	if m.todoIsSpinning {
		inProgressIcon = m.todoSpinner.View()
	}

	var pills []string
	if hasGoal {
		pills = append(pills, goalPill(m.currentGoal, inProgressIcon, goalFocused, m.pillsExpanded, t))
	}
	if hasIncomplete {
		pills = append(pills, todoPill(m.session.Todos, inProgressIcon, todosFocused, m.pillsExpanded, t))
	}
	if hasQueue {
		pills = append(pills, queuePill(m.promptQueue, queueFocused, m.pillsExpanded, t))
	}

	var expandedList string
	if m.pillsExpanded {
		if goalFocused && hasGoal {
			expandedList = t.Sidebar.WorkingDir.Render(m.currentGoal.Objective)
		} else if todosFocused && hasIncomplete {
			expandedList = todoList(m.session.Todos, inProgressIcon, t, contentWidth)
		} else if queueFocused && hasQueue {
			if m.com != nil && m.com.Workspace != nil && m.com.Workspace.AgentIsReady() {
				queueItems := m.com.Workspace.AgentQueuedPromptsList(m.session.ID)
				expandedList = queueList(queueItems, t)
			}
		}
	}

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
