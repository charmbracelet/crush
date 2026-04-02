package chat

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// -----------------------------------------------------------------------------
// Todos Tool
// -----------------------------------------------------------------------------

// TodosToolMessageItem is a message item that represents a todos tool call.
type TodosToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*TodosToolMessageItem)(nil)

// NewTodosToolMessageItem creates a new [TodosToolMessageItem].
func NewTodosToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &TodosToolRenderContext{}, canceled)
}

// TodosToolRenderContext renders todos tool messages.
type TodosToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (t *TodosToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "To-Do", opts.Anim, opts.Compact)
	}

	var params tools.TodosParams
	var meta tools.TodosResponseMetadata
	var headerText string
	var body string

	// Parse params for pending state (before result is available).
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err == nil {
		completedCount := 0
		inProgressTask := ""
		for _, todo := range params.Todos {
			if todo.Status == "completed" {
				completedCount++
			}
			if todo.Status == "in_progress" {
				if todo.ActiveForm != "" {
					inProgressTask = todo.ActiveForm
				} else {
					inProgressTask = todo.Content
				}
			}
		}

		// Default display from params (used when pending or no metadata).
		ratio := sty.Tool.TodoRatio.Render(fmt.Sprintf("%d/%d", completedCount, len(params.Todos)))
		headerText = ratio
		if inProgressTask != "" {
			headerText = fmt.Sprintf("%s · %s", ratio, inProgressTask)
		}

		// If we have metadata, use it for richer display.
		if opts.HasResult() && opts.Result.Metadata != "" {
			if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil {
				headerText, body = renderTodosMetadata(sty, meta, cappedWidth)
			}
		}
	}

	toolParams := []string{headerText}
	header := toolHeader(sty, opts.Status, "To-Do", cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if body == "" {
		return header
	}

	return joinToolParts(header, sty.Tool.Body.Render(body))
}

func renderTodosMetadata(sty *styles.Styles, meta tools.TodosResponseMetadata, width int) (string, string) {
	ratio := sty.Tool.TodoRatio.Render(fmt.Sprintf("%d/%d", meta.Completed, meta.Total))

	switch meta.Action {
	case "get":
		if meta.Current == nil {
			return ratio, ""
		}
		return fmt.Sprintf("%s · %s", ratio, meta.Current.ID), formatTodoDetails(sty, *meta.Current, width)
	case "list":
		return ratio, formatStructuredTodosList(sty, meta.Todos, width)
	case "delete":
		if meta.DeletedID == "" {
			return ratio, ""
		}
		return fmt.Sprintf("%s%s", ratio, sty.Subtle.Render(" · deleted task")), sty.Subtle.Render(meta.DeletedID)
	case "create", "update":
		if meta.Current != nil {
			head := fmt.Sprintf("%s · %d%%", ratio, meta.Current.Progress)
			return head, formatTodoDetails(sty, *meta.Current, width)
		}
	}

	if meta.IsNew {
		if meta.JustStarted != "" {
			return fmt.Sprintf("created %d todos, starting first", meta.Total), FormatTodosList(sty, meta.Todos, styles.ArrowRightIcon, width)
		}
		return fmt.Sprintf("created %d todos", meta.Total), FormatTodosList(sty, meta.Todos, styles.ArrowRightIcon, width)
	}

	hasCompleted := len(meta.JustCompleted) > 0
	hasStarted := meta.JustStarted != ""
	allCompleted := meta.Completed == meta.Total
	headerText := ratio
	if hasCompleted && hasStarted {
		headerText = fmt.Sprintf("%s%s", ratio, sty.Subtle.Render(fmt.Sprintf(" · completed %d, starting next", len(meta.JustCompleted))))
	} else if hasCompleted {
		text := sty.Subtle.Render(fmt.Sprintf(" · completed %d", len(meta.JustCompleted)))
		if allCompleted {
			text = sty.Subtle.Render(" · completed all")
		}
		headerText = fmt.Sprintf("%s%s", ratio, text)
	} else if hasStarted {
		headerText = fmt.Sprintf("%s%s", ratio, sty.Subtle.Render(" · starting task"))
	}

	if allCompleted {
		return headerText, FormatTodosList(sty, meta.Todos, styles.ArrowRightIcon, width)
	}
	if meta.Current != nil {
		return headerText, formatTodoDetails(sty, *meta.Current, width)
	}
	if meta.JustStarted != "" {
		return headerText, sty.Tool.TodoInProgressIcon.Render(styles.ArrowRightIcon+" ") + sty.Base.Render(meta.JustStarted)
	}
	return headerText, ""
}

func formatStructuredTodosList(sty *styles.Styles, todos []session.Todo, width int) string {
	if len(todos) == 0 {
		return ""
	}
	lines := make([]string, 0, len(todos))
	for _, todo := range todos {
		line := fmt.Sprintf("%s · %d%% · %s", todo.ID, todo.Progress, todo.Content)
		lines = append(lines, ansi.Truncate(sty.Base.Render(line), width, "…"))
	}
	return strings.Join(lines, "\n")
}

func formatTodoDetails(sty *styles.Styles, todo session.Todo, width int) string {
	lines := []string{
		fmt.Sprintf("%s · %s · %d%%", todo.ID, todo.Status, todo.Progress),
		todo.Content,
	}
	if todo.Status == session.TodoStatusInProgress && todo.ActiveForm != "" {
		lines = append(lines, todo.ActiveForm)
	}
	if todo.CompletedAt != 0 {
		lines = append(lines, fmt.Sprintf("completed_at=%d", todo.CompletedAt))
		lines = append(lines, fmt.Sprintf("updated_at=%d", todo.UpdatedAt))
	} else {
		lines = append(lines, fmt.Sprintf("updated_at=%d", todo.UpdatedAt))
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(sty.Base.Render(line), width, "…")
	}
	return strings.Join(lines, "\n")
}

// FormatTodosList formats a list of todos for display.
func FormatTodosList(sty *styles.Styles, todos []session.Todo, inProgressIcon string, width int) string {
	if len(todos) == 0 {
		return ""
	}

	sorted := make([]session.Todo, len(todos))
	copy(sorted, todos)
	sortTodos(sorted)

	var lines []string
	for _, todo := range sorted {
		var prefix string
		textStyle := sty.Base

		switch todo.Status {
		case session.TodoStatusCompleted:
			prefix = sty.Tool.TodoCompletedIcon.Render(styles.TodoCompletedIcon) + " "
		case session.TodoStatusInProgress:
			prefix = sty.Tool.TodoInProgressIcon.Render(inProgressIcon + " ")
		default:
			prefix = sty.Tool.TodoPendingIcon.Render(styles.TodoPendingIcon) + " "
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

// sortTodos sorts todos by status: completed, in_progress, pending.
func sortTodos(todos []session.Todo) {
	slices.SortStableFunc(todos, func(a, b session.Todo) int {
		return statusOrder(a.Status) - statusOrder(b.Status)
	})
}

// statusOrder returns the sort order for a todo status.
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
