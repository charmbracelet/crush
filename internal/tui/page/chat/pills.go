package chat

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat/todos"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

func hasIncompleteTodos(todos []session.Todo) bool {
	for _, todo := range todos {
		if todo.Status != session.TodoStatusCompleted {
			return true
		}
	}
	return false
}

const (
	pillHeightWithBorder  = 3
	maxTaskDisplayLength  = 40
	maxQueueDisplayLength = 60
)

func queuePill(queue int, focused, pillsPanelFocused bool, t *styles.Theme) string {
	if queue <= 0 {
		return ""
	}
	triangles := styles.ForegroundGrad("▶▶▶▶▶▶▶▶▶", false, t.RedDark, t.Accent)
	if queue < 10 {
		triangles = triangles[:queue]
	}

	content := fmt.Sprintf("%s %d Queued", strings.Join(triangles, ""), queue)

	style := t.S().Base.PaddingLeft(1).PaddingRight(1)
	if !pillsPanelFocused || focused {
		style = style.BorderStyle(lipgloss.RoundedBorder()).BorderForeground(t.BgOverlay)
	} else {
		style = style.BorderStyle(lipgloss.HiddenBorder())
	}
	return style.Render(content)
}

func todoPill(todos []session.Todo, spinnerView string, focused, pillsPanelFocused bool, t *styles.Theme) string {
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

	label := "To-Do"
	progress := t.S().Base.Foreground(t.FgMuted).Render(fmt.Sprintf("%d/%d", completed, total))

	var content string
	if pillsPanelFocused {
		content = fmt.Sprintf("%s %s", label, progress)
	} else if currentTodo != nil {
		taskText := currentTodo.Content
		if currentTodo.ActiveForm != "" {
			taskText = currentTodo.ActiveForm
		}
		if len(taskText) > maxTaskDisplayLength {
			taskText = taskText[:maxTaskDisplayLength-1] + "…"
		}
		task := t.S().Base.Foreground(t.FgSubtle).Render(taskText)
		content = fmt.Sprintf("%s %s %s  %s", spinnerView, label, progress, task)
	} else {
		content = fmt.Sprintf("%s %s", label, progress)
	}

	style := t.S().Base.PaddingLeft(1).PaddingRight(1)
	if !pillsPanelFocused || focused {
		style = style.BorderStyle(lipgloss.RoundedBorder()).BorderForeground(t.BgOverlay)
	} else {
		style = style.BorderStyle(lipgloss.HiddenBorder())
	}
	return style.Render(content)
}

func todoList(sessionTodos []session.Todo, spinnerView string, t *styles.Theme, width int) string {
	return todos.FormatTodosList(sessionTodos, spinnerView, t, width)
}

func queueList(queueItems []string, t *styles.Theme) string {
	if len(queueItems) == 0 {
		return ""
	}

	var lines []string
	for _, item := range queueItems {
		text := item
		if len(text) > maxQueueDisplayLength {
			text = text[:maxQueueDisplayLength-1] + "…"
		}
		prefix := t.S().Base.Foreground(t.FgMuted).Render("  •") + " "
		lines = append(lines, prefix+t.S().Base.Foreground(t.FgMuted).Render(text))
	}

	return strings.Join(lines, "\n")
}

func sectionLine(availableWidth int, t *styles.Theme) string {
	if availableWidth <= 0 {
		return ""
	}
	line := strings.Repeat("─", availableWidth)
	return t.S().Base.Foreground(t.Border).Render(line)
}
