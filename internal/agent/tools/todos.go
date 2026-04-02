package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
)

//go:embed todos.md
var todosDescription []byte

const TodosToolName = "todos"

type TodosParams struct {
	Action string     `json:"action,omitempty" description:"Action to perform: replace, create, update, delete, get, or list. Defaults to replace for backward compatibility."`
	Todos  []TodoItem `json:"todos,omitempty" description:"Todo items used by replace, create, or update actions"`
	ID     string     `json:"id,omitempty" description:"Task ID for get, update, or delete actions"`
}

type TodoItem struct {
	ID          string `json:"id,omitempty" description:"Stable task identifier for update/delete-safe operations"`
	Content     string `json:"content,omitempty" description:"What needs to be done (imperative form)"`
	Status      string `json:"status,omitempty" description:"Task status: pending, in_progress, or completed"`
	ActiveForm  string `json:"active_form,omitempty" description:"Present continuous form (e.g., 'Running tests')"`
	Progress    int    `json:"progress,omitempty" description:"Task progress percentage from 0 to 100"`
	CreatedAt   int64  `json:"created_at,omitempty" description:"Unix timestamp when the task was created"`
	UpdatedAt   int64  `json:"updated_at,omitempty" description:"Unix timestamp when the task was last updated"`
	StartedAt   int64  `json:"started_at,omitempty" description:"Unix timestamp when the task entered in_progress"`
	CompletedAt int64  `json:"completed_at,omitempty" description:"Unix timestamp when the task was completed"`
}

type TodosResponseMetadata struct {
	Action        string         `json:"action,omitempty"`
	IsNew         bool           `json:"is_new"`
	Todos         []session.Todo `json:"todos"`
	Current       *session.Todo  `json:"current,omitempty"`
	AffectedID    string         `json:"affected_id,omitempty"`
	DeletedID     string         `json:"deleted_id,omitempty"`
	JustCompleted []string       `json:"just_completed,omitempty"`
	JustStarted   string         `json:"just_started,omitempty"`
	Completed     int            `json:"completed"`
	Total         int            `json:"total"`
}

func NewTodosTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		TodosToolName,
		string(todosDescription),
		func(ctx context.Context, params TodosParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for managing todos")
			}

			currentSession, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			action := normalizeTodosAction(params.Action)
			beforeTodos := cloneTodos(currentSession.Todos)
			metadata := TodosResponseMetadata{Action: action, IsNew: len(beforeTodos) == 0}

			switch action {
			case "replace":
				updatedTodos, err := buildTodosFromReplace(params.Todos, beforeTodos)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				currentSession.Todos = updatedTodos
			case "create":
				createdTodos, err := buildCreatedTodos(params.Todos)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				currentSession.Todos = append(currentSession.Todos, createdTodos...)
			case "update":
				updatedTodos, affectedID, err := applyTodoUpdate(currentSession.Todos, params.ID, params.Todos)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				currentSession.Todos = updatedTodos
				metadata.AffectedID = affectedID
			case "delete":
				updatedTodos, deletedTodo, err := deleteTodoByID(currentSession.Todos, params.ID)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				currentSession.Todos = updatedTodos
				metadata.AffectedID = deletedTodo.ID
				metadata.DeletedID = deletedTodo.ID
			case "get":
				foundTodo, err := getTodoByID(currentSession.Todos, params.ID)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				metadata.AffectedID = foundTodo.ID
				metadata.Current = cloneTodoPtr(foundTodo)
				metadata.Todos = cloneTodos(currentSession.Todos)
				metadata.Completed, metadata.Total = summarizeTodoCounts(currentSession.Todos)
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(formatTodoResponse(foundTodo)), metadata), nil
			case "list":
				metadata.Todos = cloneTodos(currentSession.Todos)
				metadata.Completed, metadata.Total = summarizeTodoCounts(currentSession.Todos)
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(formatTodoListResponse(currentSession.Todos)), metadata), nil
			default:
				return fantasy.ToolResponse{}, fmt.Errorf("unsupported todos action %q", params.Action)
			}

			savedSession, err := sessions.Save(ctx, currentSession)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to save todos: %w", err)
			}

			metadata.Todos = cloneTodos(savedSession.Todos)
			metadata.Completed, metadata.Total = summarizeTodoCounts(savedSession.Todos)
			metadata.JustCompleted, metadata.JustStarted = summarizeTodoTransition(beforeTodos, savedSession.Todos)
			if metadata.AffectedID != "" {
				if current, ok := findTodoByID(savedSession.Todos, metadata.AffectedID); ok {
					metadata.Current = cloneTodoPtr(current)
				}
			}

			response := formatTodoMutationResponse(action, metadata)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
		},
	)
}

func normalizeTodosAction(action string) string {
	normalized := strings.ToLower(strings.TrimSpace(action))
	if normalized == "" {
		return "replace"
	}
	return normalized
}

func cloneTodos(todos []session.Todo) []session.Todo {
	if len(todos) == 0 {
		return []session.Todo{}
	}
	cloned := make([]session.Todo, len(todos))
	copy(cloned, todos)
	return cloned
}

func cloneTodoPtr(todo session.Todo) *session.Todo {
	cloned := todo
	return &cloned
}

func summarizeTodoCounts(todos []session.Todo) (completed int, total int) {
	for _, todo := range todos {
		if todo.Status == session.TodoStatusCompleted {
			completed++
		}
	}
	return completed, len(todos)
}

func summarizeTodoTransition(before, after []session.Todo) ([]string, string) {
	beforeByID := make(map[string]session.Todo, len(before))
	for _, todo := range before {
		beforeByID[todo.ID] = todo
	}

	justCompleted := make([]string, 0)
	justStarted := ""
	for _, todo := range after {
		previous, existed := beforeByID[todo.ID]
		if todo.Status == session.TodoStatusCompleted && (!existed || previous.Status != session.TodoStatusCompleted) {
			justCompleted = append(justCompleted, todo.Content)
		}
		if todo.Status == session.TodoStatusInProgress && (!existed || previous.Status != session.TodoStatusInProgress) {
			if todo.ActiveForm != "" {
				justStarted = todo.ActiveForm
			} else {
				justStarted = todo.Content
			}
		}
	}
	return justCompleted, justStarted
}

func resolveTodoStatus(status string, fallback session.TodoStatus, content string, required bool) (session.TodoStatus, error) {
	normalized := strings.TrimSpace(status)
	switch normalized {
	case "pending", "in_progress", "completed":
		return session.NormalizeTodoStatus(normalized), nil
	case "":
		if required {
			return "", fmt.Errorf("status is required for todo %q", content)
		}
		return fallback, nil
	default:
		return "", fmt.Errorf("invalid status %q for todo %q", status, content)
	}
}

func buildTodosFromReplace(items []TodoItem, existing []session.Todo) ([]session.Todo, error) {
	if len(items) == 0 {
		return []session.Todo{}, nil
	}

	now := time.Now().Unix()
	byID := make(map[string]session.Todo, len(existing))
	byContent := make(map[string]session.Todo, len(existing))
	for _, todo := range existing {
		if todo.ID != "" {
			byID[todo.ID] = todo
		}
		if todo.Content != "" {
			byContent[todo.Content] = todo
		}
	}

	todos := make([]session.Todo, len(items))
	for i, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			return nil, fmt.Errorf("content is required for todo at index %d", i)
		}
		status, err := resolveTodoStatus(item.Status, session.TodoStatusPending, content, true)
		if err != nil {
			return nil, err
		}

		base, ok := byID[strings.TrimSpace(item.ID)]
		if !ok {
			base = byContent[content]
		}
		todos[i] = buildReplacedTodo(base, item, status, now)
	}
	return todos, nil
}

func buildCreatedTodos(items []TodoItem) ([]session.Todo, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one todo is required")
	}

	now := time.Now().Unix()
	created := make([]session.Todo, len(items))
	for i, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			return nil, fmt.Errorf("content is required for todo at index %d", i)
		}
		status, err := resolveTodoStatus(item.Status, session.TodoStatusPending, content, false)
		if err != nil {
			return nil, err
		}
		created[i] = buildCreatedTodo(item, status, now)
	}
	return created, nil
}

func applyTodoUpdate(todos []session.Todo, requestedID string, items []TodoItem) ([]session.Todo, string, error) {
	if len(items) != 1 {
		return nil, "", fmt.Errorf("update action requires exactly one todo item")
	}

	item := items[0]
	targetID := strings.TrimSpace(requestedID)
	if targetID == "" {
		targetID = strings.TrimSpace(item.ID)
	}
	if targetID == "" {
		return nil, "", fmt.Errorf("todo ID is required for update")
	}

	index := -1
	for i, todo := range todos {
		if todo.ID == targetID {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, "", fmt.Errorf("todo %q not found", targetID)
	}

	status, err := resolveTodoStatus(item.Status, todos[index].Status, fallbackTodoContent(item.Content, todos[index].Content), false)
	if err != nil {
		return nil, "", err
	}

	updated := cloneTodos(todos)
	updated[index] = buildUpdatedTodo(todos[index], item, status, time.Now().Unix())
	return updated, targetID, nil
}

func deleteTodoByID(todos []session.Todo, requestedID string) ([]session.Todo, session.Todo, error) {
	targetID := strings.TrimSpace(requestedID)
	if targetID == "" {
		return nil, session.Todo{}, fmt.Errorf("todo ID is required for delete")
	}

	index := -1
	for i, todo := range todos {
		if todo.ID == targetID {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, session.Todo{}, fmt.Errorf("todo %q not found", targetID)
	}

	deleted := todos[index]
	updated := append(cloneTodos(todos[:index]), todos[index+1:]...)
	return updated, deleted, nil
}

func getTodoByID(todos []session.Todo, requestedID string) (session.Todo, error) {
	targetID := strings.TrimSpace(requestedID)
	if targetID == "" {
		return session.Todo{}, fmt.Errorf("todo ID is required")
	}
	if todo, ok := findTodoByID(todos, targetID); ok {
		return todo, nil
	}
	return session.Todo{}, fmt.Errorf("todo %q not found", targetID)
}

func findTodoByID(todos []session.Todo, id string) (session.Todo, bool) {
	for _, todo := range todos {
		if todo.ID == id {
			return todo, true
		}
	}
	return session.Todo{}, false
}

func buildReplacedTodo(base session.Todo, item TodoItem, status session.TodoStatus, now int64) session.Todo {
	progress := item.Progress
	if progress == 0 && base.ID != "" && status == base.Status {
		progress = base.Progress
	}
	progress = normalizeToolTodoProgress(status, progress)

	createdAt := firstNonZero(item.CreatedAt, base.CreatedAt, now)
	updatedAt := firstNonZero(item.UpdatedAt, now)
	startedAt := firstNonZero(item.StartedAt, base.StartedAt)
	completedAt := int64(0)
	if status == session.TodoStatusInProgress || status == session.TodoStatusCompleted {
		startedAt = firstNonZero(startedAt, now)
	}
	if status == session.TodoStatusCompleted {
		completedAt = firstNonZero(item.CompletedAt, base.CompletedAt, now)
	}

	id := strings.TrimSpace(item.ID)
	if id == "" {
		id = strings.TrimSpace(base.ID)
	}

	return session.Todo{
		ID:          id,
		Content:     strings.TrimSpace(item.Content),
		Status:      status,
		ActiveForm:  strings.TrimSpace(item.ActiveForm),
		Progress:    progress,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}
}

func buildCreatedTodo(item TodoItem, status session.TodoStatus, now int64) session.Todo {
	progress := normalizeToolTodoProgress(status, item.Progress)
	startedAt := item.StartedAt
	if status == session.TodoStatusInProgress || status == session.TodoStatusCompleted {
		startedAt = firstNonZero(startedAt, now)
	}
	completedAt := int64(0)
	if status == session.TodoStatusCompleted {
		completedAt = firstNonZero(item.CompletedAt, now)
	}

	return session.Todo{
		ID:          strings.TrimSpace(item.ID),
		Content:     strings.TrimSpace(item.Content),
		Status:      status,
		ActiveForm:  strings.TrimSpace(item.ActiveForm),
		Progress:    progress,
		CreatedAt:   firstNonZero(item.CreatedAt, now),
		UpdatedAt:   firstNonZero(item.UpdatedAt, now),
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}
}

func buildUpdatedTodo(base session.Todo, item TodoItem, status session.TodoStatus, now int64) session.Todo {
	progress := base.Progress
	if item.Progress != 0 {
		progress = item.Progress
	}
	if strings.TrimSpace(item.Status) != "" {
		switch status {
		case session.TodoStatusPending:
			progress = 0
		case session.TodoStatusCompleted:
			progress = 100
		}
	}
	progress = normalizeToolTodoProgress(status, progress)

	startedAt := firstNonZero(item.StartedAt, base.StartedAt)
	if status == session.TodoStatusInProgress || status == session.TodoStatusCompleted {
		startedAt = firstNonZero(startedAt, now)
	}
	completedAt := int64(0)
	if status == session.TodoStatusCompleted {
		completedAt = firstNonZero(item.CompletedAt, base.CompletedAt, now)
	}

	return session.Todo{
		ID:          base.ID,
		Content:     fallbackTodoContent(item.Content, base.Content),
		Status:      status,
		ActiveForm:  fallbackTodoText(item.ActiveForm, base.ActiveForm),
		Progress:    progress,
		CreatedAt:   firstNonZero(item.CreatedAt, base.CreatedAt, now),
		UpdatedAt:   firstNonZero(item.UpdatedAt, now),
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}
}

func fallbackTodoContent(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return strings.TrimSpace(fallback)
	}
	return trimmed
}

func fallbackTodoText(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return strings.TrimSpace(fallback)
	}
	return trimmed
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func normalizeToolTodoProgress(status session.TodoStatus, progress int) int {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	if status == session.TodoStatusCompleted {
		return 100
	}
	return progress
}

func formatTodoMutationResponse(action string, metadata TodosResponseMetadata) string {
	switch action {
	case "create":
		return fmt.Sprintf("Created %d tracked tasks.", metadata.Total)
	case "update":
		if metadata.Current != nil {
			return fmt.Sprintf("Updated tracked task %q (%s) to %s at %d%% progress.", metadata.Current.Content, metadata.Current.ID, metadata.Current.Status, metadata.Current.Progress)
		}
		return "Updated tracked task."
	case "delete":
		return fmt.Sprintf("Deleted tracked task %s.", metadata.DeletedID)
	case "replace":
		return fmt.Sprintf("Todo list updated successfully.\n\nStatus: %d pending, %d in progress, %d completed\nTracked tasks have been modified successfully.", countTodosByStatus(metadata.Todos, session.TodoStatusPending), countTodosByStatus(metadata.Todos, session.TodoStatusInProgress), metadata.Completed)
	default:
		return "Tracked tasks updated successfully."
	}
}

func countTodosByStatus(todos []session.Todo, status session.TodoStatus) int {
	count := 0
	for _, todo := range todos {
		if todo.Status == status {
			count++
		}
	}
	return count
}

func formatTodoResponse(todo session.Todo) string {
	return strings.Join([]string{
		fmt.Sprintf("id=%s", todo.ID),
		fmt.Sprintf("content=%s", todo.Content),
		fmt.Sprintf("status=%s", todo.Status),
		fmt.Sprintf("active_form=%s", todo.ActiveForm),
		fmt.Sprintf("progress=%d", todo.Progress),
		fmt.Sprintf("created_at=%d", todo.CreatedAt),
		fmt.Sprintf("updated_at=%d", todo.UpdatedAt),
		fmt.Sprintf("started_at=%d", todo.StartedAt),
		fmt.Sprintf("completed_at=%d", todo.CompletedAt),
	}, "\n")
}

func formatTodoListResponse(todos []session.Todo) string {
	if len(todos) == 0 {
		return "No tracked tasks found."
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "Found %d tracked tasks:\n\n", len(todos))
	for i, todo := range todos {
		fmt.Fprintf(&builder, "%d. id=%s status=%s progress=%d updated_at=%d content=%s\n", i+1, todo.ID, todo.Status, todo.Progress, todo.UpdatedAt, todo.Content)
	}
	return strings.TrimSpace(builder.String())
}
