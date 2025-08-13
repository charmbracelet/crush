package todos

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/todo"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

type RenderOptions struct {
	MaxWidth    int
	MaxItems    int
	ShowSection bool
	SectionName string
}

func RenderTodoBlock(todoService todo.Service, sessionID string, options RenderOptions, compact bool) string {
	if todoService == nil || sessionID == "" {
		return ""
	}

	ctx := context.Background()

	todos, err := todoService.List(ctx, todo.ListTodosParams{
		SessionID: sessionID,
	})
	if err != nil || len(todos) == 0 {
		return ""
	}

	t := styles.CurrentTheme()
	parts := []string{}

	if options.ShowSection {
		parts = append(parts, options.SectionName)
	}

	maxItems := options.MaxItems
	if maxItems <= 0 || maxItems > len(todos) {
		maxItems = len(todos)
	}

	for i := 0; i < maxItems; i++ {
		todoItem := todos[i]
		parts = append(parts, renderTodoItem(todoItem, options.MaxWidth, compact))
	}

	if len(todos) > maxItems {
		remaining := len(todos) - maxItems
		parts = append(parts, t.S().Muted.Render(fmt.Sprintf("... and %d more", remaining)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func renderTodoItem(todoItem todo.Todo, maxWidth int, compact bool) string {
	t := styles.CurrentTheme()

	var statusIcon string
	var statusStyle lipgloss.Style

	switch todoItem.Status {
	case todo.StatusPending:
		statusIcon = "○"
		statusStyle = t.S().Base.Foreground(t.FgMuted)
	case todo.StatusInProgress:
		statusIcon = "◐"
		statusStyle = t.S().Base.Foreground(t.Secondary)
	case todo.StatusCompleted:
		statusIcon = "●"
		statusStyle = t.S().Base.Foreground(t.Primary)
	default:
		statusIcon = "○"
		statusStyle = t.S().Base.Foreground(t.FgMuted)
	}

	content := todoItem.Content
	if maxWidth > 0 {
		iconWidth := 2
		availableWidth := maxWidth - iconWidth
		if len(content) > availableWidth {
			content = content[:availableWidth-3] + "..."
		}
	}

	contentStyle := t.S().Text
	if todoItem.Status == todo.StatusCompleted {
		contentStyle = t.S().Muted
	}

	return fmt.Sprintf("%s %s",
		statusStyle.Render(statusIcon),
		contentStyle.Render(content),
	)
}

func CountTodosByStatus(todoService todo.Service, sessionID string, status todo.Status) int64 {
	if todoService == nil || sessionID == "" {
		return 0
	}

	ctx := context.Background()
	count, err := todoService.CountBySessionAndStatus(ctx, sessionID, status)
	if err != nil {
		return 0
	}
	return count
}

func RenderTodoSummary(todoService todo.Service, sessionID string, maxWidth int) string {
	if todoService == nil || sessionID == "" {
		return ""
	}

	pending := CountTodosByStatus(todoService, sessionID, todo.StatusPending)
	inProgress := CountTodosByStatus(todoService, sessionID, todo.StatusInProgress)
	completed := CountTodosByStatus(todoService, sessionID, todo.StatusCompleted)

	total := pending + inProgress + completed
	if total == 0 {
		return ""
	}

	t := styles.CurrentTheme()
	parts := []string{}

	if pending > 0 {
		parts = append(parts, t.S().Base.Foreground(t.FgMuted).Render(fmt.Sprintf("%d pending", pending)))
	}
	if inProgress > 0 {
		parts = append(parts, t.S().Base.Foreground(t.Secondary).Render(fmt.Sprintf("%d in progress", inProgress)))
	}
	if completed > 0 {
		parts = append(parts, t.S().Base.Foreground(t.Primary).Render(fmt.Sprintf("%d completed", completed)))
	}

	summary := strings.Join(parts, " • ")
	if maxWidth > 0 && len(summary) > maxWidth {
		summary = summary[:maxWidth-3] + "..."
	}

	return summary
}
