package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

type todosSessionService struct {
	session.Service
	current   session.Session
	saveCalls int
}

func (s *todosSessionService) Get(context.Context, string) (session.Session, error) {
	return s.current, nil
}

func (s *todosSessionService) Save(_ context.Context, current session.Session) (session.Session, error) {
	s.saveCalls++
	s.current = current
	return current, nil
}

func TestTodosToolRejectsOmittedIncompleteTodoWithoutSaving(t *testing.T) {
	t.Parallel()

	current := session.Session{
		ID: "test-session",
		Todos: []session.Todo{
			{Content: "Run verification", Status: session.TodoStatusInProgress, ActiveForm: "Running verification"},
			{Content: "Fix failure", Status: session.TodoStatusPending, ActiveForm: "Fixing failure"},
		},
	}
	sessions := &todosSessionService{current: current}
	tool := NewTodosTool(sessions)
	ctx := context.WithValue(t.Context(), SessionIDContextKey, current.ID)

	_, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "call-1",
		Name:  TodosToolName,
		Input: `{"todos":[{"content":"Fix failure","status":"in_progress","active_form":"Fixing failure"}]}`,
	})

	require.EqualError(t, err, `incomplete todo "Run verification" cannot be removed`)
	require.Zero(t, sessions.saveCalls)
	require.Equal(t, current.Todos, sessions.current.Todos)
}

func TestTodosToolKeepsCompletedSnapshot(t *testing.T) {
	t.Parallel()

	sessions := &todosSessionService{current: session.Session{
		ID: "test-session",
		Todos: []session.Todo{
			{Content: "Run verification", Status: session.TodoStatusInProgress, ActiveForm: "Running verification"},
		},
	}}
	tool := NewTodosTool(sessions)
	ctx := context.WithValue(t.Context(), SessionIDContextKey, sessions.current.ID)

	response, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "call-complete",
		Name:  TodosToolName,
		Input: `{"todos":[{"content":"Run verification","status":"completed","active_form":"Running verification"}]}`,
	})
	require.NoError(t, err)
	require.Contains(t, response.Content, "All todos are completed")
	require.NotContains(t, response.Content, `{"todos":[]}`)
	require.Equal(t, []session.Todo{{
		Content:    "Run verification",
		Status:     session.TodoStatusCompleted,
		ActiveForm: "Running verification",
	}}, sessions.current.Todos)
	require.Equal(t, 1, sessions.saveCalls)
}

func TestTodosToolStartsNewListAfterCompletedSnapshot(t *testing.T) {
	t.Parallel()

	sessions := &todosSessionService{current: session.Session{
		ID: "test-session",
		Todos: []session.Todo{
			{Content: "Run verification", Status: session.TodoStatusCompleted, ActiveForm: "Running verification"},
		},
	}}
	tool := NewTodosTool(sessions)
	ctx := context.WithValue(t.Context(), SessionIDContextKey, sessions.current.ID)

	response, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "call-new-list",
		Name:  TodosToolName,
		Input: `{"todos":[{"content":"Run verification","status":"in_progress","active_form":"Running verification"}]}`,
	})
	require.NoError(t, err)

	var metadata TodosResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(response.Metadata), &metadata))
	require.True(t, metadata.IsNew)
	require.Equal(t, session.TodoStatusInProgress, sessions.current.Todos[0].Status)
}

func TestTodosToolRejectsClearingCompletedSnapshot(t *testing.T) {
	t.Parallel()

	current := session.Session{
		ID: "test-session",
		Todos: []session.Todo{
			{Content: "Run verification", Status: session.TodoStatusCompleted, ActiveForm: "Running verification"},
		},
	}
	sessions := &todosSessionService{current: current}
	tool := NewTodosTool(sessions)
	ctx := context.WithValue(t.Context(), SessionIDContextKey, current.ID)

	_, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "call-clear",
		Name:  TodosToolName,
		Input: `{"todos":[]}`,
	})
	require.EqualError(t, err, "completed todo list cannot be cleared; start a new list or leave it unchanged")
	require.Zero(t, sessions.saveCalls)
	require.Equal(t, current.Todos, sessions.current.Todos)
}

func TestTodosDescriptionExplainsUpdateSemantics(t *testing.T) {
	t.Parallel()

	require.Contains(t, todosDescription, "replaces the entire todo list")
	require.Contains(t, todosDescription, "include every existing incomplete todo")
	require.Contains(t, todosDescription, "complete all todos when the work is done")
	require.Contains(t, strings.ToLower(todosDescription), "before the final response")
	require.NotContains(t, todosDescription, "empty list")
}

func TestValidateTodoUpdateAllowsForwardStateFlow(t *testing.T) {
	t.Parallel()

	currentTodos := []session.Todo{
		{Content: "Implement feature", Status: session.TodoStatusInProgress},
		{Content: "Run tests", Status: session.TodoStatusPending},
	}
	oldStatusByContent := todoStatusByContent(currentTodos)

	err := validateTodoUpdate(currentTodos, []TodoItem{
		{Content: "Implement feature", Status: string(session.TodoStatusCompleted)},
		{Content: "Run tests", Status: string(session.TodoStatusInProgress)},
	}, oldStatusByContent)
	require.NoError(t, err)
}

func TestValidateTodoUpdateAllowsReprioritizingInProgressTodo(t *testing.T) {
	t.Parallel()

	currentTodos := []session.Todo{
		{Content: "Run verification", Status: session.TodoStatusInProgress},
		{Content: "Fix failure", Status: session.TodoStatusPending},
	}

	err := validateTodoUpdate(currentTodos, []TodoItem{
		{Content: "Run verification", Status: string(session.TodoStatusPending)},
		{Content: "Fix failure", Status: string(session.TodoStatusInProgress)},
	}, todoStatusByContent(currentTodos))
	require.NoError(t, err)
}

func TestValidateTodoUpdateAllowsAllCompleted(t *testing.T) {
	t.Parallel()

	currentTodos := []session.Todo{
		{Content: "Implement feature", Status: session.TodoStatusInProgress},
		{Content: "Run tests", Status: session.TodoStatusPending},
	}
	oldStatusByContent := todoStatusByContent(currentTodos)

	err := validateTodoUpdate(currentTodos, []TodoItem{
		{Content: "Implement feature", Status: string(session.TodoStatusCompleted)},
		{Content: "Run tests", Status: string(session.TodoStatusCompleted)},
	}, oldStatusByContent)
	require.NoError(t, err)
}

func TestValidateTodoUpdateRejectsReopeningCompletedTodo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		currentTodos []session.Todo
		nextTodos    []TodoItem
		wantErr      string
	}{
		{
			name: "completed to in progress",
			currentTodos: []session.Todo{
				{Content: "Done task", Status: session.TodoStatusCompleted},
			},
			nextTodos: []TodoItem{
				{Content: "Done task", Status: string(session.TodoStatusInProgress)},
			},
			wantErr: "cannot move back",
		},
		{
			name: "completed to pending",
			currentTodos: []session.Todo{
				{Content: "Done task", Status: session.TodoStatusCompleted},
			},
			nextTodos: []TodoItem{
				{Content: "Done task", Status: string(session.TodoStatusPending)},
			},
			wantErr: "cannot move back",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateTodoUpdate(tt.currentTodos, tt.nextTodos, todoStatusByContent(tt.currentTodos))
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidateTodoUpdateRejectsInvalidIncompleteLists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		currentTodos []session.Todo
		nextTodos    []TodoItem
		wantErr      string
	}{
		{
			name: "multiple in progress",
			nextTodos: []TodoItem{
				{Content: "First task", Status: string(session.TodoStatusInProgress)},
				{Content: "Second task", Status: string(session.TodoStatusInProgress)},
			},
			wantErr: "only one todo can be in_progress",
		},
		{
			name: "incomplete without in progress",
			nextTodos: []TodoItem{
				{Content: "Waiting task", Status: string(session.TodoStatusPending)},
			},
			wantErr: "one incomplete todo must be in_progress",
		},
		{
			name: "remove incomplete task",
			currentTodos: []session.Todo{
				{Content: "Active task", Status: session.TodoStatusInProgress},
			},
			nextTodos: []TodoItem{},
			wantErr:   "cannot be removed",
		},
		{
			name: "duplicate content",
			nextTodos: []TodoItem{
				{Content: "Same task", Status: string(session.TodoStatusInProgress)},
				{Content: "Same task", Status: string(session.TodoStatusPending)},
			},
			wantErr: "duplicate todo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateTodoUpdate(tt.currentTodos, tt.nextTodos, todoStatusByContent(tt.currentTodos))
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidateTodoUpdateRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		todos   []TodoItem
		wantErr string
	}{
		{
			name: "invalid status",
			todos: []TodoItem{
				{Content: "Bad task", Status: "blocked"},
			},
			wantErr: "invalid status",
		},
		{
			name: "empty content",
			todos: []TodoItem{
				{Status: string(session.TodoStatusInProgress)},
			},
			wantErr: "todo content is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateTodoUpdate(nil, tt.todos, nil)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func todoStatusByContent(todos []session.Todo) map[string]session.TodoStatus {
	statuses := make(map[string]session.TodoStatus, len(todos))
	for _, todo := range todos {
		statuses[todo.Content] = todo.Status
	}
	return statuses
}
