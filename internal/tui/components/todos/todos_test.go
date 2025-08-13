package todos

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/todo"
	"github.com/stretchr/testify/require"
)

// mockTodoService is a simple mock implementation for testing
type mockTodoService struct {
	todos []todo.Todo
}

func (m *mockTodoService) Create(ctx context.Context, params todo.CreateTodoParams) (todo.Todo, error) {
	newTodo := todo.Todo{
		ID:        "test-id",
		SessionID: params.SessionID,
		Content:   params.Content,
		Status:    params.Status,
		CreatedAt: 1234567890,
		UpdatedAt: 1234567890,
	}
	m.todos = append(m.todos, newTodo)
	return newTodo, nil
}

func (m *mockTodoService) Get(ctx context.Context, id string) (todo.Todo, error) {
	for _, t := range m.todos {
		if t.ID == id {
			return t, nil
		}
	}
	return todo.Todo{}, nil
}

func (m *mockTodoService) List(ctx context.Context, params todo.ListTodosParams) ([]todo.Todo, error) {
	var result []todo.Todo
	for _, t := range m.todos {
		if t.SessionID == params.SessionID {
			if params.Status == nil || t.Status == *params.Status {
				result = append(result, t)
			}
		}
	}
	return result, nil
}

func (m *mockTodoService) Update(ctx context.Context, id string, params todo.UpdateTodoParams) (todo.Todo, error) {
	for i, t := range m.todos {
		if t.ID == id {
			m.todos[i].Content = params.Content
			m.todos[i].Status = params.Status
			return m.todos[i], nil
		}
	}
	return todo.Todo{}, nil
}

func (m *mockTodoService) UpdateStatus(ctx context.Context, id string, status todo.Status) (todo.Todo, error) {
	for i, t := range m.todos {
		if t.ID == id {
			m.todos[i].Status = status
			return m.todos[i], nil
		}
	}
	return todo.Todo{}, nil
}

func (m *mockTodoService) Delete(ctx context.Context, id string) error {
	for i, t := range m.todos {
		if t.ID == id {
			m.todos = append(m.todos[:i], m.todos[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockTodoService) DeleteBySession(ctx context.Context, sessionID string) error {
	var filtered []todo.Todo
	for _, t := range m.todos {
		if t.SessionID != sessionID {
			filtered = append(filtered, t)
		}
	}
	m.todos = filtered
	return nil
}

func (m *mockTodoService) CountBySessionAndStatus(ctx context.Context, sessionID string, status todo.Status) (int64, error) {
	count := int64(0)
	for _, t := range m.todos {
		if t.SessionID == sessionID && t.Status == status {
			count++
		}
	}
	return count, nil
}

func TestRenderTodoBlock(t *testing.T) {
	t.Parallel()

	mockService := &mockTodoService{
		todos: []todo.Todo{
			{
				ID:        "1",
				SessionID: "session-1",
				Content:   "Implement feature X",
				Status:    todo.StatusPending,
			},
			{
				ID:        "2",
				SessionID: "session-1",
				Content:   "Fix bug Y",
				Status:    todo.StatusInProgress,
			},
			{
				ID:        "3",
				SessionID: "session-1",
				Content:   "Write tests",
				Status:    todo.StatusCompleted,
			},
		},
	}

	result := RenderTodoBlock(mockService, "session-1", RenderOptions{
		MaxWidth:    50,
		MaxItems:    10,
		ShowSection: true,
		SectionName: "TODOs",
	}, false)

	require.NotEmpty(t, result)
	require.Contains(t, result, "TODOs")
	require.Contains(t, result, "Implement feature X")
	require.Contains(t, result, "Fix bug Y")
	require.Contains(t, result, "Write tests")
}

func TestCountTodosByStatus(t *testing.T) {
	t.Parallel()

	mockService := &mockTodoService{
		todos: []todo.Todo{
			{SessionID: "session-1", Status: todo.StatusPending},
			{SessionID: "session-1", Status: todo.StatusPending},
			{SessionID: "session-1", Status: todo.StatusInProgress},
			{SessionID: "session-1", Status: todo.StatusCompleted},
		},
	}

	pendingCount := CountTodosByStatus(mockService, "session-1", todo.StatusPending)
	require.Equal(t, int64(2), pendingCount)

	inProgressCount := CountTodosByStatus(mockService, "session-1", todo.StatusInProgress)
	require.Equal(t, int64(1), inProgressCount)

	completedCount := CountTodosByStatus(mockService, "session-1", todo.StatusCompleted)
	require.Equal(t, int64(1), completedCount)
}

func TestRenderTodoSummary(t *testing.T) {
	t.Parallel()

	mockService := &mockTodoService{
		todos: []todo.Todo{
			{SessionID: "session-1", Status: todo.StatusPending},
			{SessionID: "session-1", Status: todo.StatusInProgress},
			{SessionID: "session-1", Status: todo.StatusCompleted},
		},
	}

	summary := RenderTodoSummary(mockService, "session-1", 100)
	require.NotEmpty(t, summary)
	// Check for the text content, ignoring ANSI color codes
	require.Contains(t, summary, "1 pending")
	require.Contains(t, summary, "1 in progress")
	require.Contains(t, summary, "1 co") // Check for truncated "completed"
}
