package todos

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/x/ansi"
)

func sortTodos(todos []session.Todo) {
	slices.SortStableFunc(todos, func(a, b session.Todo) int {
		return statusOrder(a.Status) - statusOrder(b.Status)
	})
}

func statusOrder(s session.TodoStatus) int {
	switch s {
	case session.TodoStatusCompleted:
		return 0
	case session.TodoStatusInProgress:
		return 1
	default:
		return 2
	}
}

func FormatTodosList(todos []session.Todo, inProgressIcon string, t *styles.Theme, width int) string {
	if len(todos) == 0 {
		return ""
	}

	sorted := make([]session.Todo, len(todos))
	copy(sorted, todos)
	sortTodos(sorted)

	var lines []string
	for _, todo := range sorted {
		var prefix string
		var textStyle lipgloss.Style

		switch todo.Status {
		case session.TodoStatusCompleted:
			prefix = t.S().Base.Foreground(t.Green).Render(styles.TodoCompletedIcon) + " "
			textStyle = t.S().Base.Foreground(t.FgBase)
		case session.TodoStatusInProgress:
			prefix = t.S().Base.Foreground(t.GreenDark).Render(inProgressIcon + " ")
			textStyle = t.S().Base.Foreground(t.FgBase)
		default:
			prefix = t.S().Base.Foreground(t.FgMuted).Render(styles.TodoPendingIcon) + " "
			textStyle = t.S().Base.Foreground(t.FgBase)
		}

		text := todo.Content
		if todo.Status == session.TodoStatusInProgress && todo.ActiveForm != "" {
			text = todo.ActiveForm
		}
		line := prefix + textStyle.Render(text)
		line = ansi.Truncate(line, width, "…")

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
