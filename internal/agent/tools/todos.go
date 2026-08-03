package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
)

//go:embed todos.md
var todosDescription string

const TodosToolName = "todos"

type TodosParams struct {
	Todos []TodoItem `json:"todos" description:"The updated todo list. Pass an empty array to clear the list when all work is done. A list where every item is completed is cleared automatically."`
}

type TodoItem struct {
	Content    string `json:"content" description:"What needs to be done (imperative form)"`
	Status     string `json:"status" description:"Task status: pending, in_progress, or completed"`
	ActiveForm string `json:"active_form" description:"Present continuous form (e.g., 'Running tests')"`
}

type TodosResponseMetadata struct {
	IsNew         bool           `json:"is_new"`
	Todos         []session.Todo `json:"todos"`
	JustCompleted []string       `json:"just_completed,omitempty"`
	JustStarted   string         `json:"just_started,omitempty"`
	Completed     int            `json:"completed"`
	Total         int            `json:"total"`
	// Cleared is true when the list was emptied (explicit empty update or
	// auto-clear after every item was marked completed).
	Cleared bool `json:"cleared,omitempty"`
	// AutoCleared is true when the list was cleared because every submitted
	// item was completed, rather than because the model passed [].
	AutoCleared bool `json:"auto_cleared,omitempty"`
}

func NewTodosTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		TodosToolName,
		todosDescription,
		func(ctx context.Context, params TodosParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for managing todos")
			}

			currentSession, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			hadTodos := len(currentSession.Todos) > 0
			isNew := !hadTodos
			oldStatusByContent := make(map[string]session.TodoStatus)
			for _, todo := range currentSession.Todos {
				oldStatusByContent[todo.Content] = todo.Status
			}

			for _, item := range params.Todos {
				switch item.Status {
				case "pending", "in_progress", "completed":
				default:
					return fantasy.ToolResponse{}, fmt.Errorf("invalid status %q for todo %q", item.Status, item.Content)
				}
			}

			todos := make([]session.Todo, len(params.Todos))
			var justCompleted []string
			var justStarted string
			completedCount := 0
			inProgressCount := 0

			for i, item := range params.Todos {
				todos[i] = session.Todo{
					Content:    item.Content,
					Status:     session.TodoStatus(item.Status),
					ActiveForm: item.ActiveForm,
				}

				newStatus := session.TodoStatus(item.Status)
				oldStatus, existed := oldStatusByContent[item.Content]

				switch newStatus {
				case session.TodoStatusCompleted:
					completedCount++
					if existed && oldStatus != session.TodoStatusCompleted {
						justCompleted = append(justCompleted, item.Content)
					}
				case session.TodoStatusInProgress:
					inProgressCount++
					if !existed || oldStatus != session.TodoStatusInProgress {
						if item.ActiveForm != "" {
							justStarted = item.ActiveForm
						} else {
							justStarted = item.Content
						}
					}
				}
			}

			// When every submitted item is completed, clear the list so the
			// UI pill cannot linger on a finished job. Models often mark the
			// last task completed and stop without a second clear call.
			autoCleared := len(todos) > 0 && completedCount == len(todos)
			explicitClear := hadTodos && len(todos) == 0
			// The transcript entry for the turn that finished the work
			// must still show the ticked list and "n/n". Only the
			// persisted list (which drives the pill) is cleared, so the
			// snapshot the renderer reads is captured before the reset.
			submitted := todos
			submittedCompleted := completedCount
			if autoCleared {
				todos = nil
				completedCount = 0
				inProgressCount = 0
				justStarted = ""
			}

			// Persist nil rather than empty slice so JSON/DB treat a cleared
			// list the same as a session that never had todos.
			if len(todos) == 0 {
				currentSession.Todos = nil
			} else {
				currentSession.Todos = todos
			}
			_, err = sessions.Save(ctx, currentSession)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to save todos: %w", err)
			}

			cleared := explicitClear || autoCleared
			response := buildTodosResponse(todos, completedCount, inProgressCount, cleared, autoCleared, justCompleted)

			metadata := TodosResponseMetadata{
				IsNew:         isNew,
				Todos:         submitted,
				JustCompleted: justCompleted,
				JustStarted:   justStarted,
				Completed:     submittedCompleted,
				Total:         len(submitted),
				Cleared:       cleared,
				AutoCleared:   autoCleared,
			}

			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
		},
	)
}

// buildTodosResponse writes the model-facing tool result.
func buildTodosResponse(todos []session.Todo, completedCount, inProgressCount int, cleared, autoCleared bool, justCompleted []string) string {
	if autoCleared {
		n := len(justCompleted)
		if n == 0 {
			return "All todos completed. Todo list cleared."
		}
		return fmt.Sprintf("Completed %d todo(s). All work done; todo list cleared.", n)
	}
	if cleared {
		return "Todo list cleared. All work for this list is done."
	}

	pendingCount := 0
	var inProgressItems, pendingItems []string
	for _, todo := range todos {
		switch todo.Status {
		case session.TodoStatusPending:
			pendingCount++
			pendingItems = append(pendingItems, todo.Content)
		case session.TodoStatusInProgress:
			inProgressItems = append(inProgressItems, todo.Content)
		}
	}

	response := "Todo list updated successfully.\n\n"
	response += fmt.Sprintf("Status: %d pending, %d in progress, %d completed\n",
		pendingCount, inProgressCount, completedCount)
	if len(inProgressItems) > 0 {
		response += "In progress: " + strings.Join(inProgressItems, ", ") + "\n"
	}
	if len(pendingItems) > 0 {
		response += "Remaining: " + strings.Join(pendingItems, ", ") + "\n"
	}

	switch {
	case len(todos) == 0:
		response += "Todo list is empty."
	case inProgressCount == 0 && pendingCount > 0:
		response += "No task is in_progress. Mark the next pending task in_progress before continuing, or complete remaining work when finished (a fully completed list is cleared automatically)."
	default:
		response += "Continue using the todo list to track progress. Mark the current task completed as soon as it is fully done. When every task is completed the list clears automatically."
	}
	return response
}
