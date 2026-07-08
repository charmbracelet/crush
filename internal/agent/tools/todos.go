package tools

import (
	"context"
	_ "embed"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
)

//go:embed todos.md
var todosDescription string

const TodosToolName = "todos"

type TodosParams struct {
	Todos []TodoItem `json:"todos" description:"The complete replacement todo list; include every existing incomplete todo"`
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

			isNew := !session.HasIncompleteTodos(currentSession.Todos)
			if isNew && len(currentSession.Todos) > 0 && len(params.Todos) == 0 {
				return fantasy.ToolResponse{}, fmt.Errorf("completed todo list cannot be cleared; start a new list or leave it unchanged")
			}

			currentTodos := currentSession.Todos
			if isNew {
				currentTodos = nil
			}
			oldStatusByContent := make(map[string]session.TodoStatus)
			for _, todo := range currentTodos {
				oldStatusByContent[todo.Content] = todo.Status
			}

			if err := validateTodoUpdate(currentTodos, params.Todos, oldStatusByContent); err != nil {
				return fantasy.ToolResponse{}, err
			}

			todos := make([]session.Todo, len(params.Todos))
			var justCompleted []string
			var justStarted string
			completedCount := 0

			for i, item := range params.Todos {
				todos[i] = session.Todo{
					Content:    item.Content,
					Status:     session.TodoStatus(item.Status),
					ActiveForm: item.ActiveForm,
				}

				newStatus := session.TodoStatus(item.Status)
				oldStatus, existed := oldStatusByContent[item.Content]

				if newStatus == session.TodoStatusCompleted {
					completedCount++
					if existed && oldStatus != session.TodoStatusCompleted {
						justCompleted = append(justCompleted, item.Content)
					}
				}

				if newStatus == session.TodoStatusInProgress {
					if !existed || oldStatus != session.TodoStatusInProgress {
						if item.ActiveForm != "" {
							justStarted = item.ActiveForm
						} else {
							justStarted = item.Content
						}
					}
				}
			}

			currentSession.Todos = todos
			_, err = sessions.Save(ctx, currentSession)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to save todos: %w", err)
			}

			response := "Todo list updated successfully.\n\n"

			pendingCount := 0
			inProgressCount := 0

			for _, todo := range todos {
				switch todo.Status {
				case session.TodoStatusPending:
					pendingCount++
				case session.TodoStatusInProgress:
					inProgressCount++
				}
			}

			response += fmt.Sprintf("Status: %d pending, %d in progress, %d completed\n",
				pendingCount, inProgressCount, completedCount)

			switch {
			case len(todos) == 0:
				response += "Todo list cleared successfully. You may now provide your final response."
			case completedCount == len(todos):
				response += "All todos are completed. You may now provide your final response."
			default:
				response += "Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current tasks if applicable."
			}

			metadata := TodosResponseMetadata{
				IsNew:         isNew,
				Todos:         todos,
				JustCompleted: justCompleted,
				JustStarted:   justStarted,
				Completed:     completedCount,
				Total:         len(todos),
			}

			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
		},
	)
}

func validateTodoUpdate(currentTodos []session.Todo, nextTodos []TodoItem, oldStatusByContent map[string]session.TodoStatus) error {
	inProgressCount := 0
	incompleteCount := 0
	seenContent := make(map[string]struct{}, len(nextTodos))

	for _, item := range nextTodos {
		newStatus := session.TodoStatus(item.Status)
		switch newStatus {
		case session.TodoStatusPending, session.TodoStatusInProgress, session.TodoStatusCompleted:
		default:
			return fmt.Errorf("invalid status %q for todo %q", item.Status, item.Content)
		}

		if item.Content == "" {
			return fmt.Errorf("todo content is required")
		}
		if _, ok := seenContent[item.Content]; ok {
			return fmt.Errorf("duplicate todo %q", item.Content)
		}
		seenContent[item.Content] = struct{}{}

		if newStatus != session.TodoStatusCompleted {
			incompleteCount++
		}
		if newStatus == session.TodoStatusInProgress {
			inProgressCount++
			if inProgressCount > 1 {
				return fmt.Errorf("only one todo can be in_progress")
			}
		}

		oldStatus, existed := oldStatusByContent[item.Content]
		if !existed {
			continue
		}
		if oldStatus == session.TodoStatusCompleted && newStatus != session.TodoStatusCompleted {
			return fmt.Errorf("completed todo %q cannot move back to %q", item.Content, item.Status)
		}
	}

	for _, todo := range currentTodos {
		if todo.Status == session.TodoStatusCompleted {
			continue
		}
		if _, ok := seenContent[todo.Content]; !ok {
			return fmt.Errorf("incomplete todo %q cannot be removed", todo.Content)
		}
	}
	if incompleteCount > 0 && inProgressCount == 0 {
		return fmt.Errorf("one incomplete todo must be in_progress")
	}

	return nil
}
