package todo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	return db.SetupTestDBWithData(t, func(conn *sql.DB) {
		// Insert test session
		err := db.CreateTestSession(conn, "test-session", "Test Session")
		require.NoError(t, err)
	})
}

func TestTodoService_Create(t *testing.T) {
	t.Parallel()

	conn := setupTestDB(t)
	defer conn.Close()

	service := NewService(db.New(conn))

	todo, err := service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "Test TODO item",
		Status:    StatusPending,
	})

	require.NoError(t, err)
	require.NotEmpty(t, todo.ID)
	require.Equal(t, "test-session", todo.SessionID)
	require.Equal(t, "Test TODO item", todo.Content)
	require.Equal(t, StatusPending, todo.Status)
	require.NotZero(t, todo.CreatedAt)
	require.Equal(t, todo.CreatedAt, todo.UpdatedAt)
}

func TestTodoService_Get(t *testing.T) {
	t.Parallel()

	conn := setupTestDB(t)
	defer conn.Close()

	service := NewService(db.New(conn))

	created, err := service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "Test TODO item",
		Status:    StatusPending,
	})
	require.NoError(t, err)

	retrieved, err := service.Get(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created, retrieved)
}

func TestTodoService_List(t *testing.T) {
	t.Parallel()

	conn := setupTestDB(t)
	defer conn.Close()

	service := NewService(db.New(conn))

	// Create test TODOs
	todo1, err := service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "TODO 1",
		Status:    StatusPending,
	})
	require.NoError(t, err)

	_, err = service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "TODO 2",
		Status:    StatusInProgress,
	})
	require.NoError(t, err)

	_, err = service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "TODO 3",
		Status:    StatusCompleted,
	})
	require.NoError(t, err)

	// Test listing all TODOs for session
	todos, err := service.List(context.Background(), ListTodosParams{
		SessionID: "test-session",
	})
	require.NoError(t, err)
	require.Len(t, todos, 3)

	// Test filtering by status
	status := StatusPending
	todos, err = service.List(context.Background(), ListTodosParams{
		SessionID: "test-session",
		Status:    &status,
	})
	require.NoError(t, err)
	require.Len(t, todos, 1)
	require.Equal(t, todo1.ID, todos[0].ID)
}

func TestTodoService_Update(t *testing.T) {
	t.Parallel()

	conn := setupTestDB(t)
	defer conn.Close()

	service := NewService(db.New(conn))

	created, err := service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "Original content",
		Status:    StatusPending,
	})
	require.NoError(t, err)

	time.Sleep(1 * time.Millisecond) // Ensure different timestamp

	updated, err := service.Update(context.Background(), created.ID, UpdateTodoParams{
		Content: "Updated content",
		Status:  StatusInProgress,
	})
	require.NoError(t, err)

	require.Equal(t, created.ID, updated.ID)
	require.Equal(t, "Updated content", updated.Content)
	require.Equal(t, StatusInProgress, updated.Status)
	require.True(t, updated.UpdatedAt > created.UpdatedAt)
}

func TestTodoService_UpdateStatus(t *testing.T) {
	t.Parallel()

	conn := setupTestDB(t)
	defer conn.Close()

	service := NewService(db.New(conn))

	created, err := service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "Test content",
		Status:    StatusPending,
	})
	require.NoError(t, err)

	time.Sleep(1 * time.Millisecond) // Ensure different timestamp

	updated, err := service.UpdateStatus(context.Background(), created.ID, StatusCompleted)
	require.NoError(t, err)

	require.Equal(t, created.ID, updated.ID)
	require.Equal(t, created.Content, updated.Content)
	require.Equal(t, StatusCompleted, updated.Status)
	require.True(t, updated.UpdatedAt > created.UpdatedAt)
}

func TestTodoService_Delete(t *testing.T) {
	t.Parallel()

	conn := setupTestDB(t)
	defer conn.Close()

	service := NewService(db.New(conn))

	created, err := service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "Test content",
		Status:    StatusPending,
	})
	require.NoError(t, err)

	err = service.Delete(context.Background(), created.ID)
	require.NoError(t, err)

	// Verify it's deleted
	_, err = service.Get(context.Background(), created.ID)
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}

func TestTodoService_CountBySessionAndStatus(t *testing.T) {
	t.Parallel()

	conn := setupTestDB(t)
	defer conn.Close()

	service := NewService(db.New(conn))

	// Create TODOs with different statuses
	_, err := service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "Pending TODO 1",
		Status:    StatusPending,
	})
	require.NoError(t, err)

	_, err = service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "Pending TODO 2",
		Status:    StatusPending,
	})
	require.NoError(t, err)

	_, err = service.Create(context.Background(), CreateTodoParams{
		SessionID: "test-session",
		Content:   "In Progress TODO",
		Status:    StatusInProgress,
	})
	require.NoError(t, err)

	// Count pending TODOs
	count, err := service.CountBySessionAndStatus(context.Background(), "test-session", StatusPending)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	// Count in-progress TODOs
	count, err = service.CountBySessionAndStatus(context.Background(), "test-session", StatusInProgress)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	// Count completed TODOs (should be 0)
	count, err = service.CountBySessionAndStatus(context.Background(), "test-session", StatusCompleted)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}
