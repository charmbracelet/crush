package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

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

func TestValidateTodoUpdateRejectsBackwardStateFlow(t *testing.T) {
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
		{
			name: "in progress to pending",
			currentTodos: []session.Todo{
				{Content: "Active task", Status: session.TodoStatusInProgress},
			},
			nextTodos: []TodoItem{
				{Content: "Active task", Status: string(session.TodoStatusPending)},
			},
			wantErr: "cannot move back to pending",
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
